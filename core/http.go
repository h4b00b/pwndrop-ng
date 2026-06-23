package core

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

	// Wrap kind decides Content-Type / Content-Disposition before anything
	// else writes to the response — both override the file's normal MIME.
	var wrap wrapKind
	var wrapped bool
	if status == "ok" {
		wrap, wrapped = wrapInfo(f.WrapAs)
	}
	if wrapped {
		mime_type = wrap.contentType
		dispName := wrappedFilename(f.Name, wrap)
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, dispName))
		w.Header().Set("X-Content-Wrapped", strings.ToLower(f.WrapAs))
	}
	w.Header().Set("Content-Type", mime_type)
	w.Header().Set("X-Content-Type-Options", "nosniff")

	// Watermark: append a per-download tag so a leaked sample maps back to
	// IP/UA/timestamp. Only on real serves, never on the paused facade. With
	// watermark on, the served bytes differ from the stored blob, so the
	// SHA256/Digest headers would lie — suppress them and surface an
	// X-Content-Watermarked flag instead.
	var watermarkTag string
	var watermarkSuffix []byte
	if status == "ok" && f.Watermark {
		watermarkTag = utils.GenRandomString(32)
		watermarkSuffix = []byte("\x00PWN:" + watermarkTag + "\n")
	}
	if watermarkTag != "" {
		w.Header().Set("X-Content-Watermarked", "true")
	}
	// SHA256 / Digest only reflect the stored blob; with watermark or wrap
	// the served bytes differ, so we don't advertise a hash we can't honour.
	if status == "ok" && !wrapped && watermarkTag == "" && f.SHA256 != "" {
		// Advertise the stored-blob hash so the target can verify what they
		// fetched. Skip when watermarking — different bytes every fetch. Skip
		// for the paused facade — those bytes aren't the blob the SHA describes.
		// Digest is RFC 3230 (base64), X-Content-SHA256 is the operator-friendly
		// hex twin.
		if raw, err := hex.DecodeString(f.SHA256); err == nil && len(raw) == 32 {
			w.Header().Set("X-Content-SHA256", f.SHA256)
			w.Header().Set("Digest", "sha-256="+base64.StdEncoding.EncodeToString(raw))
		}
	}

	// Range / Resume serving — opt-in via DbConfig.RangeEnabled and only safe
	// when no per-file policy ties downloads to a single body delivery. Quota
	// would count each partial as a hit; burn-after-read would fire on the
	// first range request and orphan the resume. Facade serving never honors
	// Range because the sub-file bytes are intentionally throwaway. Watermark
	// breaks Range too — partial fetches would not see the appended suffix.
	useRange := !needsLock && status == "ok" && watermarkTag == "" && !wrapped && cfg != nil && cfg.RangeEnabled
	if useRange {
		rangeStatus := status
		if r.Header.Get("Range") != "" {
			rangeStatus = "ok-range"
		}
		// Zero modtime suppresses Last-Modified / If-Modified-Since handling —
		// we want every fetch logged, not served from the client cache. The
		// name arg is only used by ServeContent for Content-Type sniffing,
		// which we've already overridden via the Header().Set above.
		http.ServeContent(w, r, f.Name, time.Time{}, fo)
		logBlock(f, r, from_ip, rangeStatus, muted)
		return
	}

	if wrapped {
		// Wrap path: container is built around the blob (and any watermark
		// suffix) and streamed. Content-Length is unknown ahead of time, so
		// we let Go fall back to chunked transfer-encoding. Counter / burn
		// behaviour mirrors the raw path — but with no FileSize ground-truth
		// we settle for "no copy error" as the delivery signal.
		w.WriteHeader(200)
		n, werr := serveWrapped(w, f.WrapAs, fo, f.Name, watermarkSuffix)
		if werr != nil {
			log.Error("http: wrap: %s: wrote %d err=%v (%s)", f.Filename, n, werr, from_ip)
			logBlock(f, r, from_ip, "aborted-wrap", muted)
			return
		}
		if cnt, err := storage.FileIncrementDownloads(f.ID); err == nil {
			if f.MaxDownloads > 0 && cnt >= f.MaxDownloads {
				storage.FileEnable(f.ID, false)
			}
		}
		logBlockWatermark(f, r, from_ip, status+"-wrap", watermarkTag, muted)
		if f.BurnAfterRead && status == "ok" {
			fo.Close()
			BurnFile(f.ID)
		}
		return
	}

	if watermarkTag != "" {
		// Set Content-Length explicitly so clients with strict length checks
		// don't trip on the extra suffix bytes.
		w.Header().Set("Content-Length", fmt.Sprintf("%d", f.FileSize+int64(len(watermarkSuffix))))
	}
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
	// Append the watermark suffix AFTER the full body so the client gets the
	// blob plus tag in one contiguous response. If the suffix write fails,
	// we still log the tag — the operator at least knows it was minted and
	// (probably) reached the wire.
	if len(watermarkSuffix) > 0 {
		if _, err := w.Write(watermarkSuffix); err != nil {
			log.Error("http: watermark write: %s (%s)", err, from_ip)
		}
	}
	if n, err := storage.FileIncrementDownloads(f.ID); err == nil {
		if f.MaxDownloads > 0 && n >= f.MaxDownloads {
			storage.FileEnable(f.ID, false)
		}
	}
	logBlockWatermark(f, r, from_ip, status, watermarkTag, muted)
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
