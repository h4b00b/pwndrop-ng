package core

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/kgretzky/pwndrop/log"
	"github.com/kgretzky/pwndrop/storage"
	"github.com/kgretzky/pwndrop/utils"
)

const BLACKLIST_JAIL_TIME_SECS = 10 * 60
const BLACKLIST_HITS_LIMIT = 10

type BlacklistItem struct {
	hits     int
	last_hit time.Time
}

type Http struct {
	srv *Server
}

func NewHttp(srv *Server) (*Http, error) {
	s := &Http{
		srv: srv,
	}
	return s, nil
}

func (s *Http) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		softNotFound(w)
		return
	}

	data_dir := Cfg.GetDataDir()
	cfg, _ := storage.ConfigGet(1)
	trustProxy := cfg != nil && cfg.TrustProxy
	from_ip := utils.ClientIP(r, trustProxy)

	f, action := s.srv.runGate(w, r, from_ip, cfg, true)
	if action == gateBlocked {
		return
	}
	if f == nil {
		log.Error("http: get: %s: no file (%s)", r.URL.Path, from_ip)
		softNotFound(w)
		return
	}
	muted := f.NotifyMuted

	if f.RedirectPath != "" && f.RedirectPath != r.URL.Path && !f.IsPaused {
		log.Error("http: get: %s: redirecting to '%s' (%s)", r.URL.Path, f.RedirectPath, from_ip)
		logBlock(f, r, from_ip, "redirect", muted)
		http.Redirect(w, r, f.RedirectPath, http.StatusFound)
		return
	}

	// Files with quota or burn-after-read need a per-file lock around the
	// (re-validate, write body, increment, burn) sequence — otherwise
	// concurrent requests race past the quota check on a stale snapshot, or
	// both serve a burn-once payload. Under the lock we re-run the FULL gate
	// so a mid-flight password change / new filter / kill switch flip also
	// catches the request that already passed the first gate.
	needsLock := f.MaxDownloads > 0 || f.BurnAfterRead
	if needsLock {
		release := lockFile(f.ID)
		defer release()
		// countHits=false: the first runGate above already credited the
		// matching rule; a second hit here would double-count.
		fresh, act2 := s.srv.runGate(w, r, from_ip, cfg, false)
		if act2 == gateBlocked {
			return
		}
		if fresh == nil {
			logBlock(f, r, from_ip, "gone", muted)
			softNotFound(w)
			return
		}
		f = fresh
	}

	mime_type := f.MimeType
	status := "ok"
	if f.IsPaused {
		mime_type = f.SubMimeType
		status = "paused-facade"
	}
	fpath := filepath.Join(data_dir, "files", f.Filename)
	fo, err := os.Open(fpath)
	if err != nil {
		log.Error("http: file: %s: %s (%s)", f.Filename, err, from_ip)
		softNotFound(w)
		return
	}
	defer fo.Close()

	w.Header().Set("Content-Type", mime_type)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(200)
	written, copyErr := io.Copy(w, fo)
	// Only credit a real delivery: scanner GETs that hang up after headers,
	// client TCP-RST mid-body, or Range-griefed partial fetches must not
	// consume the quota, log as "ok", or trigger burn-after-read. We're still
	// under the per-file lock here, so a concurrent waiter sees the bumped
	// counter (or the burn) before it gets its own snapshot.
	delivered := copyErr == nil && written >= f.FileSize
	if !delivered {
		log.Error("http: copy: %s: wrote %d/%d err=%v (%s)", f.Filename, written, f.FileSize, copyErr, from_ip)
		logBlock(f, r, from_ip, "aborted", muted)
		return
	}
	if n, err := storage.FileIncrementDownloads(f.ID); err == nil {
		if f.MaxDownloads > 0 && n >= f.MaxDownloads {
			storage.FileEnable(f.ID, false)
		}
	}
	logBlock(f, r, from_ip, status, muted)
	// Burn-after-read: nuke the record + blob once the body is fully written.
	// Guarded by status="ok" so a paused-facade serve does not consume the
	// burn. Close the handle first so the blob delete succeeds on Windows
	// where open files are locked.
	if f.BurnAfterRead && status == "ok" {
		fo.Close()
		BurnFile(f.ID)
	}
}

// softNotFound writes a plain 404 with no body. Use this for "request looked
// legit but resolves to nothing right now" cases (file disabled/expired/quota
// hit, filter deny, password mismatch, no facade) so pwndrop's response looks
// like an ordinary not-found instead of the hijack-and-cut pattern that scans
// can fingerprint. killConnection is reserved for kill-switch / blacklist
// where actively tearing the TCP is the point.
func softNotFound(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusNotFound)
}

// serveFacade writes the file's sub-file payload as if the file were paused.
// Returns false when no facade is attached so the caller can fall back to 404.
func (s *Http) serveFacade(w http.ResponseWriter, f *storage.DbFile, dataDir string) bool {
	if f.RefSubFile <= 0 {
		return false
	}
	sf, err := storage.SubFileGet(f.RefSubFile)
	if err != nil {
		return false
	}
	fpath := filepath.Join(dataDir, "files", sf.Filename)
	fo, err := os.Open(fpath)
	if err != nil {
		return false
	}
	defer fo.Close()
	mime := f.SubMimeType
	if mime == "" {
		mime = "application/octet-stream"
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(200)
	io.Copy(w, fo)
	return true
}

func (s *Http) killConnection(w http.ResponseWriter, status int) error {
	if status > 0 {
		w.Header().Set("Connection", "close")
		w.Header().Set("Content-Length", "0")
		w.WriteHeader(status)
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		return fmt.Errorf("connection hijacking not supported")
	}
	conn, _, err := hj.Hijack()
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}
