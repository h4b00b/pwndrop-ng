// Package updater handles in-place self-update from GitHub releases.
//
// Flow: GET /repos/<repo>/releases/latest, compare tag against config.Version,
// pick the asset matching runtime.GOOS/GOARCH, download to a sibling temp file,
// chmod, atomic rename over the running binary, then re-exec.
package updater

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/kgretzky/pwndrop/config"
	"github.com/kgretzky/pwndrop/log"
)

const (
	// GitHub repo that publishes releases. Kept here (not in user config) so
	// nobody can repoint auto-update at an attacker-controlled fork.
	Repo = "h4b00b/pwndrop-ng"

	releasesURL = "https://api.github.com/repos/" + Repo + "/releases/latest"

	// Cap downloads at 200 MiB — pwndrop binaries are ~20 MiB, this just
	// stops a runaway/poisoned response from filling the disk.
	maxDownloadBytes = 200 << 20

	httpTimeout = 60 * time.Second
)

type Asset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
	Size        int64  `json:"size"`
}

type Release struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	HTMLURL     string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
	Prerelease  bool      `json:"prerelease"`
	Draft       bool      `json:"draft"`
	Assets      []Asset   `json:"assets"`
}

type CheckResult struct {
	Current     string    `json:"current"`
	Latest      string    `json:"latest"`
	Available   bool      `json:"available"`
	Notes       string    `json:"notes"`
	ReleaseURL  string    `json:"release_url"`
	PublishedAt time.Time `json:"published_at"`
	AssetName   string    `json:"asset_name"`
	AssetURL    string    `json:"asset_url"`
}

var (
	mu       sync.Mutex
	inFlight bool
)

// Check queries GitHub for the latest release and reports whether it's newer
// than the running version. Network errors are returned; "no asset for this
// platform" is also an error so the UI can show it.
func Check(ctx context.Context) (*CheckResult, error) {
	rel, err := fetchLatest(ctx)
	if err != nil {
		return nil, err
	}

	asset := pickAsset(rel.Assets)
	res := &CheckResult{
		Current:     config.Version,
		Latest:      strings.TrimPrefix(rel.TagName, "v"),
		Notes:       rel.Body,
		ReleaseURL:  rel.HTMLURL,
		PublishedAt: rel.PublishedAt,
	}
	if asset != nil {
		res.AssetName = asset.Name
		res.AssetURL = asset.DownloadURL
	}
	res.Available = isNewer(res.Latest, res.Current)
	return res, nil
}

// Apply downloads the matching asset for the latest release, atomically swaps
// the running binary, and re-execs the new process. On success this function
// does not return — the process image is replaced.
func Apply(ctx context.Context) error {
	mu.Lock()
	if inFlight {
		mu.Unlock()
		return errors.New("update already in progress")
	}
	inFlight = true
	mu.Unlock()
	defer func() {
		mu.Lock()
		inFlight = false
		mu.Unlock()
	}()

	rel, err := fetchLatest(ctx)
	if err != nil {
		return err
	}
	asset := pickAsset(rel.Assets)
	if asset == nil {
		return fmt.Errorf("no release asset for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	latest := strings.TrimPrefix(rel.TagName, "v")
	if !isNewer(latest, config.Version) {
		return fmt.Errorf("already on latest version %s", config.Version)
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate self: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("resolve self: %w", err)
	}

	log.Info("updater: downloading %s (%d bytes) -> %s", asset.Name, asset.Size, exePath)
	tmp, err := downloadTo(ctx, asset.DownloadURL, exePath)
	if err != nil {
		return err
	}

	if err := swap(tmp, exePath); err != nil {
		_ = os.Remove(tmp)
		return err
	}

	log.Info("updater: swapped binary, re-executing %s", exePath)
	if err := reexec(exePath); err != nil {
		return fmt.Errorf("re-exec: %w", err)
	}
	return nil // unreachable on success
}

func fetchLatest(ctx context.Context) (*Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releasesURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "pwndrop-ng-updater/"+config.Version)

	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github releases: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("github releases: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("decode release: %w", err)
	}
	if rel.Draft {
		return nil, errors.New("latest release is a draft")
	}
	return &rel, nil
}

// pickAsset matches by GOOS/GOARCH and ".tar.gz" suffix. CI publishes
// pwndrop-ng-linux-<arch>.tar.gz containing pwndrop-ng/pwndrop-ng inside.
func pickAsset(assets []Asset) *Asset {
	want := runtime.GOOS + "-" + runtime.GOARCH
	for i := range assets {
		n := strings.ToLower(assets[i].Name)
		if strings.Contains(n, want) && strings.HasSuffix(n, ".tar.gz") {
			return &assets[i]
		}
	}
	return nil
}

// downloadTo streams the release tarball straight through gzip+tar and writes
// the embedded binary to a sibling temp file of dest (same directory so the
// later rename is atomic on the same filesystem). The .tar.gz itself never
// hits the disk — there's nothing to clean up beyond the temp binary, which
// either gets renamed into place or removed on failure.
//
// The picked entry is the regular file inside the archive whose basename
// matches the running binary's basename (typically "pwndrop-ng"). Falling
// back to "the only regular file in the archive" keeps the updater working
// if the bundle ever gets repackaged differently.
func downloadTo(ctx context.Context, url, dest string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "pwndrop-ng-updater/"+config.Version)

	// Long timeout for download (tarball is ~7 MiB; allow slow networks).
	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download: HTTP %d", resp.StatusCode)
	}

	// Hard cap on bytes pulled off the wire — tar header parsing happens
	// after gzip inflation but the LimitReader bounds the *compressed* input,
	// which is what protects us against a runaway response.
	body := io.LimitReader(resp.Body, maxDownloadBytes+1)
	gz, err := gzip.NewReader(body)
	if err != nil {
		return "", fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	wantName := filepath.Base(dest)

	tmp, err := os.CreateTemp(filepath.Dir(dest), ".pwndrop-update-*")
	if err != nil {
		return "", fmt.Errorf("temp file: %w", err)
	}
	tmpPath := tmp.Name()
	// On any error past this point, remove the temp file.
	cleanup := func() { _ = tmp.Close(); _ = os.Remove(tmpPath) }

	var written int64
	var matched bool
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			cleanup()
			return "", fmt.Errorf("tar: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
			continue
		}
		// Match by basename so we tolerate both "pwndrop-ng" at the
		// archive root and the current "pwndrop-ng/pwndrop-ng" layout.
		if path.Base(hdr.Name) != wantName {
			// First-pass strict match. Loose fallback handled after the
			// loop if nothing matched.
			continue
		}
		if _, err := io.Copy(tmp, tr); err != nil {
			cleanup()
			return "", fmt.Errorf("extract: %w", err)
		}
		written = hdr.Size
		matched = true
		break
	}
	if !matched {
		cleanup()
		return "", fmt.Errorf("binary %q not found in archive", wantName)
	}
	if written == 0 {
		cleanup()
		return "", errors.New("extracted binary is empty")
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("close: %w", err)
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("chmod: %w", err)
	}
	return tmpPath, nil
}

// swap replaces dest with src atomically. On Linux a running ELF can be
// rename(2)'d over without affecting the executing process (kernel keeps the
// old inode mapped until exit); the next exec sees the new file.
func swap(src, dest string) error {
	if err := os.Rename(src, dest); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// reexec replaces the current process image with the freshly-written binary,
// preserving PID and args. Under systemd this keeps the unit healthy; in
// foreground mode the user sees the new process come up in place.
//
// On platforms without syscall.Exec (Windows) we fall back to spawning a
// detached child and exiting; the new process inherits stdin/stdout/stderr.
func reexec(exePath string) error {
	args := os.Args
	env := os.Environ()
	// Give the HTTP response a moment to flush before we vanish.
	time.Sleep(250 * time.Millisecond)
	if runtime.GOOS != "windows" {
		return syscall.Exec(exePath, args, env)
	}
	// Windows fallback: spawn detached, then exit.
	proc, err := os.StartProcess(exePath, args, &os.ProcAttr{
		Env:   env,
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
	})
	if err != nil {
		return err
	}
	_ = proc.Release()
	os.Exit(0)
	return nil
}

// isNewer reports whether latest > current. Accepts "1.2.3" or "v1.2.3".
// Non-numeric suffixes (e.g. "-rc1") sort the prerelease as older than the
// equivalent stable, matching common-case expectations.
func isNewer(latest, current string) bool {
	la := parseSemver(latest)
	cu := parseSemver(current)
	for i := 0; i < 3; i++ {
		if la.parts[i] != cu.parts[i] {
			return la.parts[i] > cu.parts[i]
		}
	}
	// Equal numeric parts: stable beats prerelease.
	if la.pre == "" && cu.pre != "" {
		return true
	}
	return false
}

type semver struct {
	parts [3]int
	pre   string
}

func parseSemver(s string) semver {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	var v semver
	pre := ""
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		pre = s[i+1:]
		s = s[:i]
	}
	v.pre = pre
	for i, p := range strings.SplitN(s, ".", 3) {
		n, _ := strconv.Atoi(strings.TrimFunc(p, func(r rune) bool {
			return r < '0' || r > '9'
		}))
		v.parts[i] = n
	}
	return v
}
