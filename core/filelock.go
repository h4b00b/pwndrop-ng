package core

import "sync"

// fileMuMap maps file ID → *sync.Mutex, populated on demand. The serve path
// holds the per-file lock around the (re-read state, validate quota, write
// body, increment counter, burn) sequence to close the TOCTOU windows on
// MaxDownloads and BurnAfterRead. Files with no quota and no burn skip the
// lock entirely so they aren't serialised needlessly.
var fileMuMap sync.Map

// lockFile acquires the per-id lock and returns a release function. Safe to
// defer.
func lockFile(id int) func() {
	v, _ := fileMuMap.LoadOrStore(id, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// ForgetFileLock drops the lock entry for a file id that no longer exists.
// Call this from any code path that removes a file record (delete, burn,
// cleanup) so the lock map stays bounded over the process lifetime.
func ForgetFileLock(id int) {
	fileMuMap.Delete(id)
}
