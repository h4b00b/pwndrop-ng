package core

import (
	"os"
	"path/filepath"
	"time"

	"github.com/kgretzky/pwndrop/api"
	"github.com/kgretzky/pwndrop/log"
	"github.com/kgretzky/pwndrop/storage"
)

// StartCleanupLoop runs the periodic auto-cleanup task in the background. It
// is a no-op unless CleanupEnabled is on; the loop re-reads the config every
// tick so the operator can toggle it live from the panel without restart.
//
// Tick interval is fixed at 1h — short enough to keep tail growth bounded,
// long enough to be invisible at scale.
func StartCleanupLoop() {
	go func() {
		t := time.NewTicker(1 * time.Hour)
		defer t.Stop()
		for range t.C {
			runCleanupTick()
		}
	}()
}

// RunCleanupNow forces a single cleanup pass — used by the API "run now"
// button so the operator can see the effect immediately instead of waiting up
// to an hour for the next tick.
func RunCleanupNow() {
	runCleanupTick()
}

func runCleanupTick() {
	// Stale chunked-upload sweep runs unconditionally — the operator-facing
	// CleanupEnabled toggle gates user-data expiry (a different concern),
	// not internal temp-blob hygiene.
	if n := api.ChunkedSweepStale(api.ChunkedDefaultTTL()); n > 0 {
		log.Info("cleanup: swept %d stale chunked uploads", n)
	}
	// Same justification for the rate-limit bucket map — internal memory
	// hygiene, runs regardless of the user-facing toggle.
	if n := RateLimitSweepIdle(); n > 0 {
		log.Debug("cleanup: swept %d idle rate-limit buckets", n)
	}

	cfg, err := storage.ConfigGet(1)
	if err != nil || !cfg.CleanupEnabled {
		return
	}

	if cfg.CleanupExpiredAfterDays > 0 {
		cleanupExpiredFiles(cfg.CleanupExpiredAfterDays)
	}
	if cfg.CleanupLogMaxEntries > 0 {
		cleanupOldLogs(cfg.CleanupLogMaxEntries)
	}
}

// cleanupExpiredFiles deletes files whose ExpireAt was hit more than `days`
// ago. We don't touch active files or files without an expiry set.
func cleanupExpiredFiles(days int) {
	cutoff := time.Now().Unix() - int64(days)*86400
	files, err := storage.FileList()
	if err != nil {
		log.Error("cleanup: list files: %s", err)
		return
	}
	dataDir := Cfg.GetDataDir()
	deleted := 0
	for _, f := range files {
		if f.ExpireAt <= 0 || f.ExpireAt > cutoff {
			continue
		}
		// Mirror the FileDeleteHandler cleanup: facade, password, per-file
		// filters, file row, on-disk blob.
		if f.RefSubFile > 0 {
			if sf, err := storage.SubFileGet(f.RefSubFile); err == nil {
				os.Remove(filepath.Join(dataDir, "files", sf.Filename))
				_ = storage.SubFileDelete(f.RefSubFile)
			}
		}
		_ = storage.FileDelete(f.ID)
		_ = storage.FilePasswordDelete(f.ID)
		_ = storage.FilterDeleteForFile(f.ID)
		os.Remove(filepath.Join(dataDir, "files", f.Filename))
		ForgetFileLock(f.ID)
		deleted++
	}
	if deleted > 0 {
		log.Info("cleanup: deleted %d expired files (older than %d days past expiry)", deleted, days)
	}
}

// BurnFile deletes a file record + on-disk blob + facade + password +
// per-file filters in a single shot. Used by the burn-after-read enforcement
// in http.go after a successful download. Mirrors the cleanup logic in
// cleanupExpiredFiles and api.deleteFileFull. Errors are swallowed because
// the response has already been written when the burn fires — best-effort.
func BurnFile(id int) {
	f, err := storage.FileGet(id)
	if err != nil {
		log.Error("burn: get %d: %s", id, err)
		return
	}
	dataDir := Cfg.GetDataDir()
	if f.RefSubFile > 0 {
		if sf, err := storage.SubFileGet(f.RefSubFile); err == nil {
			os.Remove(filepath.Join(dataDir, "files", sf.Filename))
			_ = storage.SubFileDelete(f.RefSubFile)
		}
	}
	_ = storage.FileDelete(f.ID)
	_ = storage.FilePasswordDelete(f.ID)
	_ = storage.FilterDeleteForFile(f.ID)
	os.Remove(filepath.Join(dataDir, "files", f.Filename))
	ForgetFileLock(f.ID)
	log.Info("burn: deleted file id=%d url=%s", id, f.UrlPath)
}

// cleanupOldLogs trims the download log to the most recent `maxEntries`. We
// list everything (the log is per-event but small per row), keep the newest
// N by timestamp, delete the rest.
func cleanupOldLogs(maxEntries int) {
	all, err := storage.DownloadLogList(0)
	if err != nil {
		log.Error("cleanup: list logs: %s", err)
		return
	}
	if len(all) <= maxEntries {
		return
	}
	// DownloadLogList returns newest-first; cut from index maxEntries onward.
	toDelete := all[maxEntries:]
	for _, e := range toDelete {
		_ = storage.DownloadLogDelete(e.ID)
	}
	log.Info("cleanup: trimmed %d old download log entries", len(toDelete))
}
