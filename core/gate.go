package core

import (
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/kgretzky/pwndrop/filter"
	"github.com/kgretzky/pwndrop/log"
	"github.com/kgretzky/pwndrop/storage"
)

type gateAction int

const (
	gateAllow gateAction = iota
	gateBlocked
)

// runGate is the shared pre-serve enforcement chain. Runs:
// kill switch → file lookup → expire → quota → filter chain → per-file password.
//
// Returns:
//   - (file, gateAllow)  — caller may proceed; serve the file.
//   - (nil, gateAllow)   — url did not resolve to a known file; caller decides
//     (HTTP: 404; WebDAV: pass through to dav handler for dir ops).
//   - (nil, gateBlocked) — response already written (404 / 302 / 401 / facade);
//     caller must return immediately.
//
// countHits=false skips the FilterIncrementHits side-effect — used by the
// second runGate call we do under the per-file lock so a request against a
// quota/burn file does not double-count its rule match.
//
// Used by both the HTTP serve path and the WebDAV serve path so that scanners
// cannot bypass kill switch / filters / password by spoofing the WebDAV UA.
func (s *Server) runGate(w http.ResponseWriter, r *http.Request, fromIp string, cfg *storage.DbConfig, countHits bool) (*storage.DbFile, gateAction) {
	if cfg != nil && cfg.KillSwitch {
		logBlock(nil, r, fromIp, "kill-switch", false)
		s.killConnection(w, 404)
		return nil, gateBlocked
	}

	f, _, err := s.GetFile(r.URL.Path)
	if err != nil {
		return nil, gateAllow
	}
	muted := f.NotifyMuted

	if f.ExpireAt > 0 && time.Now().Unix() >= f.ExpireAt {
		log.Error("http: get: %s: link expired (%s)", r.URL.Path, fromIp)
		logBlock(f, r, fromIp, "expired", muted)
		softNotFound(w)
		return nil, gateBlocked
	}
	if f.MaxDownloads > 0 && f.DownloadCount >= f.MaxDownloads {
		log.Error("http: get: %s: download quota reached (%s)", r.URL.Path, fromIp)
		logBlock(f, r, fromIp, "exhausted", muted)
		softNotFound(w)
		return nil, gateBlocked
	}

	if dec, on := filter.Evaluate(r, f.ID, fromIp); on {
		if countHits && dec.RuleId > 0 {
			storage.FilterIncrementHits(dec.RuleId)
		}
		switch dec.Action {
		case filter.ActionDeny:
			logBlock(f, r, fromIp, dec.LogTag(), muted)
			softNotFound(w)
			return nil, gateBlocked
		case filter.ActionRedirect:
			logBlock(f, r, fromIp, dec.LogTag(), muted)
			if url := Cfg.GetRedirectUrl(); url != "" {
				http.Redirect(w, r, url, http.StatusFound)
			} else {
				softNotFound(w)
			}
			return nil, gateBlocked
		case filter.ActionFacade:
			logBlock(f, r, fromIp, dec.LogTag(), muted)
			if !s.http.serveFacade(w, f, Cfg.GetDataDir()) {
				softNotFound(w)
			}
			return nil, gateBlocked
		}
	}

	if pwHash := storage.FileGetPasswordHash(f.ID); pwHash != "" {
		_, supplied, ok := r.BasicAuth()
		if !ok || bcrypt.CompareHashAndPassword([]byte(pwHash), []byte(supplied)) != nil {
			logBlock(f, r, fromIp, "bad-password", muted)
			w.Header().Set("WWW-Authenticate", `Basic realm="pwndrop"`)
			w.WriteHeader(http.StatusUnauthorized)
			return nil, gateBlocked
		}
	}

	return f, gateAllow
}

// logBlock builds the standard DownloadLog event for a gate block / success
// case. f may be nil (kill switch hits before file lookup); the other call
// sites always have a file at hand.
func logBlock(f *storage.DbFile, r *http.Request, fromIp, status string, muted bool) {
	ev := &storage.DbDownloadLog{
		UrlPath:   r.URL.Path,
		RemoteIp:  fromIp,
		UserAgent: r.Header.Get("User-Agent"),
		Referer:   r.Header.Get("Referer"),
		Status:    status,
	}
	if f != nil {
		ev.FileId = f.ID
		ev.FileName = f.Name
	}
	LogDownload(ev, muted)
}
