package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/mux"

	"github.com/kgretzky/pwndrop/log"
	"github.com/kgretzky/pwndrop/storage"
	"github.com/kgretzky/pwndrop/utils"
)

// Chunked upload protocol — sidesteps the 100 MB request-body cap that
// Cloudflare's free plan applies to every proxied request. Browser slices the
// file into chunks under the cap, posts each one separately, then asks the
// server to finalize.
//
// Flow:
//   POST /api/v1/files/chunked/init        -> {upload_id, chunk_size}
//   POST /api/v1/files/chunked/{id}        body = raw chunk
//                                          header X-Chunk-Offset = byte offset
//   POST /api/v1/files/chunked/{id}/complete   -> finalize, returns DbFile
//   DELETE /api/v1/files/chunked/{id}      abort, drop temp blob
//
// In-progress uploads live in process memory only — a restart aborts every
// half-uploaded blob. Acceptable: the admin retries from the browser. Stale
// entries (>chunkedUploadTTL with no activity) get reaped by the cleanup
// loop via ChunkedSweepStale.

const (
	// ChunkedSuggestedSize is what we tell the client to use per chunk. Comfortably
	// below the 100 MB CF free-plan body cap with room for headers/overhead.
	ChunkedSuggestedSize int64 = 50 << 20 // 50 MiB

	// chunkedMaxChunkBody is the hard ceiling on a single chunk POST body —
	// generous enough that a client that ignores our suggested size and picks
	// 90 MiB still works, but tight enough that a buggy/malicious client can't
	// stream gigabytes into one append call.
	chunkedMaxChunkBody int64 = 95 << 20 // 95 MiB

	// chunkedMaxTotal is the cap on a full chunked upload. Mostly a sanity
	// guard against an accidental TB upload eating the disk.
	chunkedMaxTotal int64 = 50 << 30 // 50 GiB

	// chunkedUploadTTL is how long an inactive upload sticks around before
	// the cleanup sweep drops it.
	chunkedUploadTTL = 24 * time.Hour
)

type chunkedUpload struct {
	ID         string
	Name       string
	MimeType   string
	TotalSize  int64
	Received   int64
	TempPath   string
	CreateTime time.Time
	LastSeen   time.Time
	mu         sync.Mutex
}

var (
	chunkedReg   = map[string]*chunkedUpload{}
	chunkedRegMu sync.Mutex
)

func chunkedDir() string {
	return filepath.Join(Cfg.GetDataDir(), "files", ".chunked")
}

func chunkedGet(id string) *chunkedUpload {
	chunkedRegMu.Lock()
	defer chunkedRegMu.Unlock()
	return chunkedReg[id]
}

func chunkedDrop(id string) {
	chunkedRegMu.Lock()
	u := chunkedReg[id]
	delete(chunkedReg, id)
	chunkedRegMu.Unlock()
	if u != nil {
		os.Remove(u.TempPath)
	}
}

func ChunkedOptionsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Methods", "POST,DELETE,OPTIONS")
}

func ChunkedInitHandler(w http.ResponseWriter, r *http.Request) {
	if _, err := AuthSession(r); err != nil {
		DumpResponse(w, "unauthorized", http.StatusUnauthorized, API_ERROR_BAD_AUTHENTICATION, nil)
		return
	}

	limitBody(w, r, MaxJSONBody)
	type req struct {
		Name      string `json:"name"`
		MimeType  string `json:"mime_type"`
		TotalSize int64  `json:"total_size"`
	}
	j := req{}
	if err := json.NewDecoder(r.Body).Decode(&j); err != nil {
		DumpResponse(w, err.Error(), http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
		return
	}
	if j.TotalSize <= 0 || j.TotalSize > chunkedMaxTotal {
		DumpResponse(w, "invalid total_size", http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
		return
	}
	name := utils.SanitizeUrlSegment(j.Name)
	mime := j.MimeType
	if mime == "" {
		mime = "application/octet-stream"
	}

	id := utils.GenRandomHash()
	if err := os.MkdirAll(chunkedDir(), 0700); err != nil {
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_SAVE_FAILED, nil)
		return
	}
	tempPath := filepath.Join(chunkedDir(), id)
	f, err := os.OpenFile(tempPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_SAVE_FAILED, nil)
		return
	}
	f.Close()

	now := time.Now()
	u := &chunkedUpload{
		ID:         id,
		Name:       name,
		MimeType:   mime,
		TotalSize:  j.TotalSize,
		TempPath:   tempPath,
		CreateTime: now,
		LastSeen:   now,
	}
	chunkedRegMu.Lock()
	chunkedReg[id] = u
	chunkedRegMu.Unlock()

	type resp struct {
		UploadID  string `json:"upload_id"`
		ChunkSize int64  `json:"chunk_size"`
	}
	DumpResponse(w, "ok", http.StatusOK, 0, resp{UploadID: id, ChunkSize: ChunkedSuggestedSize})
}

func ChunkedAppendHandler(w http.ResponseWriter, r *http.Request) {
	if _, err := AuthSession(r); err != nil {
		DumpResponse(w, "unauthorized", http.StatusUnauthorized, API_ERROR_BAD_AUTHENTICATION, nil)
		return
	}

	id := mux.Vars(r)["id"]
	u := chunkedGet(id)
	if u == nil {
		DumpResponse(w, "upload not found", http.StatusNotFound, API_ERROR_FILE_NOT_FOUND, nil)
		return
	}

	// Strict append-only: the client tells us the byte offset it's writing
	// from, and we reject anything that isn't exactly the current Received.
	// Catches reorder/duplicate/retry-after-partial-success without server
	// having to seek + truncate.
	offStr := r.Header.Get("X-Chunk-Offset")
	if offStr == "" {
		DumpResponse(w, "missing X-Chunk-Offset", http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
		return
	}
	off, err := strconv.ParseInt(offStr, 10, 64)
	if err != nil || off < 0 {
		DumpResponse(w, "invalid X-Chunk-Offset", http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
		return
	}

	limitBody(w, r, chunkedMaxChunkBody)

	u.mu.Lock()
	defer u.mu.Unlock()

	if off != u.Received {
		DumpResponse(w, fmt.Sprintf("offset mismatch: got %d want %d", off, u.Received), http.StatusConflict, API_ERROR_BAD_REQUEST, map[string]int64{"received": u.Received})
		return
	}

	f, err := os.OpenFile(u.TempPath, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_SAVE_FAILED, nil)
		return
	}
	defer f.Close()

	// Cap how much we accept this call to whatever remains of the declared
	// total — protects against a client streaming past the size it committed
	// to at init.
	remaining := u.TotalSize - u.Received
	n, copyErr := io.CopyN(f, r.Body, remaining+1) // +1 to detect overrun
	if copyErr != nil && !errors.Is(copyErr, io.EOF) {
		// MaxBytesReader / network error / overrun all land here. We still
		// committed `n` bytes to disk — keep them so the client can resume.
		u.Received += n
		u.LastSeen = time.Now()
		DumpResponse(w, copyErr.Error(), http.StatusBadRequest, API_ERROR_BAD_REQUEST, map[string]int64{"received": u.Received})
		return
	}
	if n > remaining {
		// Client tried to send past the declared total. Truncate the temp
		// back to the limit so complete can still succeed.
		_ = f.Truncate(u.TotalSize)
		u.Received = u.TotalSize
		u.LastSeen = time.Now()
		DumpResponse(w, "overrun", http.StatusBadRequest, API_ERROR_BAD_REQUEST, map[string]int64{"received": u.Received})
		return
	}
	u.Received += n
	u.LastSeen = time.Now()
	DumpResponse(w, "ok", http.StatusOK, 0, map[string]int64{"received": u.Received})
}

func ChunkedCompleteHandler(w http.ResponseWriter, r *http.Request) {
	if _, err := AuthSession(r); err != nil {
		DumpResponse(w, "unauthorized", http.StatusUnauthorized, API_ERROR_BAD_AUTHENTICATION, nil)
		return
	}

	id := mux.Vars(r)["id"]
	u := chunkedGet(id)
	if u == nil {
		DumpResponse(w, "upload not found", http.StatusNotFound, API_ERROR_FILE_NOT_FOUND, nil)
		return
	}

	u.mu.Lock()
	defer u.mu.Unlock()

	if u.Received != u.TotalSize {
		DumpResponse(w, fmt.Sprintf("incomplete: %d/%d", u.Received, u.TotalSize), http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
		return
	}

	// Promote temp blob into the regular files dir under a fresh random
	// filename, the same way FileCreateHandler does after SaveUploadedFile.
	dataDir := Cfg.GetDataDir()
	os.Mkdir(filepath.Join(dataDir, "files"), 0700)
	finalName := utils.GenRandomHash()
	finalPath := filepath.Join(dataDir, "files", finalName)
	if err := os.Rename(u.TempPath, finalPath); err != nil {
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_SAVE_FAILED, nil)
		return
	}

	fi, err := os.Stat(finalPath)
	if err != nil {
		os.Remove(finalPath)
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_NOT_FOUND, nil)
		return
	}

	o := &storage.DbFile{
		Uid:          1,
		Name:         u.Name,
		Filename:     finalName,
		FileSize:     fi.Size(),
		UrlPath:      "/" + utils.GenRandomString(8) + "/" + u.Name,
		RedirectPath: "",
		MimeType:     u.MimeType,
		SubMimeType:  u.MimeType,
		OrigMimeType: u.MimeType,
		CreateTime:   time.Now().Unix(),
		IsEnabled:    true,
		IsPaused:     false,
		RefSubFile:   0,
	}
	f, err := storage.FileCreate(o)
	if err != nil {
		os.Remove(finalPath)
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_DATABASE_FAILED, nil)
		return
	}

	chunkedRegMu.Lock()
	delete(chunkedReg, id)
	chunkedRegMu.Unlock()

	log.Debug("chunked upload complete: %s (%d bytes)", u.Name, fi.Size())
	DumpResponse(w, "ok", http.StatusOK, 0, f)
}

// ChunkedReplaceCompleteHandler finalizes a chunked upload as a replacement
// blob for an existing file. Mirrors FileReplaceHandler's behavior: swaps the
// on-disk blob, updates Filename/FileSize/MimeType, keeps URL/policy/password/
// filters, resets DownloadCount, deletes the old blob.
func ChunkedReplaceCompleteHandler(w http.ResponseWriter, r *http.Request) {
	if _, err := AuthSession(r); err != nil {
		DumpResponse(w, "unauthorized", http.StatusUnauthorized, API_ERROR_BAD_AUTHENTICATION, nil)
		return
	}

	fid, err := strconv.Atoi(mux.Vars(r)["fid"])
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
		return
	}
	existing, err := storage.FileGet(fid)
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusNotFound, API_ERROR_FILE_NOT_FOUND, nil)
		return
	}

	id := mux.Vars(r)["id"]
	u := chunkedGet(id)
	if u == nil {
		DumpResponse(w, "upload not found", http.StatusNotFound, API_ERROR_FILE_NOT_FOUND, nil)
		return
	}

	u.mu.Lock()
	defer u.mu.Unlock()

	if u.Received != u.TotalSize {
		DumpResponse(w, fmt.Sprintf("incomplete: %d/%d", u.Received, u.TotalSize), http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
		return
	}

	dataDir := Cfg.GetDataDir()
	oldBlob := filepath.Join(dataDir, "files", existing.Filename)
	newName := utils.GenRandomHash()
	newBlob := filepath.Join(dataDir, "files", newName)
	if err := os.Rename(u.TempPath, newBlob); err != nil {
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_SAVE_FAILED, nil)
		return
	}
	fi, err := os.Stat(newBlob)
	if err != nil {
		os.Remove(newBlob)
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_NOT_FOUND, nil)
		return
	}

	mime := u.MimeType
	if mime == "" {
		mime = existing.MimeType
	}
	displayName := existing.Name
	if u.Name != "" {
		displayName = u.Name
	}

	if err := storage.FileReplaceBlob(fid, newName, fi.Size(), mime, mime, displayName); err != nil {
		os.Remove(newBlob)
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_DATABASE_FAILED, nil)
		return
	}
	_ = storage.FileResetDownloadCount(fid)
	_ = os.Remove(oldBlob)

	chunkedRegMu.Lock()
	delete(chunkedReg, id)
	chunkedRegMu.Unlock()

	final, _ := storage.FileGet(fid)
	DumpResponse(w, "ok", http.StatusOK, 0, final)
}

func ChunkedAbortHandler(w http.ResponseWriter, r *http.Request) {
	if _, err := AuthSession(r); err != nil {
		DumpResponse(w, "unauthorized", http.StatusUnauthorized, API_ERROR_BAD_AUTHENTICATION, nil)
		return
	}
	id := mux.Vars(r)["id"]
	chunkedDrop(id)
	DumpResponse(w, "ok", http.StatusOK, 0, nil)
}

// ChunkedSweepStale drops any in-flight uploads whose LastSeen is older than
// maxAge. Called from the cleanup loop so half-uploaded blobs don't pile up
// on disk forever when a client walks away mid-upload.
func ChunkedSweepStale(maxAge time.Duration) int {
	cutoff := time.Now().Add(-maxAge)
	var victims []string
	chunkedRegMu.Lock()
	for id, u := range chunkedReg {
		if u.LastSeen.Before(cutoff) {
			victims = append(victims, id)
		}
	}
	chunkedRegMu.Unlock()
	for _, id := range victims {
		chunkedDrop(id)
	}
	return len(victims)
}

// ChunkedDefaultTTL exposes the package-level TTL so the cleanup caller
// doesn't need to hard-code it.
func ChunkedDefaultTTL() time.Duration { return chunkedUploadTTL }
