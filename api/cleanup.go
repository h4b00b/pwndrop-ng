package api

import "net/http"

// cleanupRunner is set by main at startup to core.RunCleanupNow. Indirected
// through a function pointer so this package doesn't have to import core
// (which would create a cycle: core already imports api for the router).
var cleanupRunner func()

// SetCleanupRunner is called from main.go to wire the API endpoint to the
// in-process cleanup function. A nil runner makes the "run now" endpoint a
// no-op — still returns 200 so the UI button stays simple.
func SetCleanupRunner(fn func()) {
	cleanupRunner = fn
}

// forgetFileLock is set by main at startup to core.ForgetFileLock. Same
// callback pattern as cleanupRunner to avoid the api→core import cycle. Used
// from the file delete paths so the per-file lock map doesn't leak entries.
var forgetFileLock func(int)

// SetForgetFileLock wires the per-file lock drop callback from main.go.
func SetForgetFileLock(fn func(int)) {
	forgetFileLock = fn
}

// dropFileLock invokes the registered callback if any. Safe to call when no
// runner is wired (tests, partial startup).
func dropFileLock(id int) {
	if forgetFileLock != nil {
		forgetFileLock(id)
	}
}

func CleanupRunHandler(w http.ResponseWriter, r *http.Request) {
	if _, err := AuthSession(r); err != nil {
		DumpResponse(w, "unauthorized", http.StatusUnauthorized, API_ERROR_BAD_AUTHENTICATION, nil)
		return
	}
	if cleanupRunner != nil {
		cleanupRunner()
	}
	DumpResponse(w, "ok", http.StatusOK, 0, nil)
}
