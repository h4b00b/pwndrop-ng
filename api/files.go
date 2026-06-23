package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"

	"github.com/kgretzky/pwndrop/log"
	"github.com/kgretzky/pwndrop/storage"
	"github.com/kgretzky/pwndrop/utils"
)

// adminReservedPaths and adminReservedPrefixes are the URL paths that the
// download dispatcher (core/server.go ServeHTTP) routes to the admin SPA /
// API router. A user-controlled file UrlPath/RedirectPath that lands on any
// of them would be served instead — replacing the admin login page or an
// /api/v1/* endpoint with attacker-controlled content (operator phishing).
var (
	adminReservedPaths    = []string{"/", "/index.html", "/favicon.png", "/api", "/api/v1"}
	adminReservedPrefixes = []string{"/api/v1/", "/assets/", "/img/"}
)

func validateUserUrlPath(p string) error {
	if p == "" || p[0] != '/' {
		return fmt.Errorf("path must start with /")
	}
	if strings.ContainsAny(p, "\x00\r\n\t") {
		return fmt.Errorf("path contains control characters")
	}
	if strings.Contains(p, "//") || strings.Contains(p, "/../") || strings.HasSuffix(p, "/..") {
		return fmt.Errorf("path contains invalid segments")
	}
	for _, r := range adminReservedPaths {
		if p == r {
			return fmt.Errorf("path %q is reserved", p)
		}
	}
	for _, pfx := range adminReservedPrefixes {
		if strings.HasPrefix(p, pfx) {
			return fmt.Errorf("path prefix %q is reserved", pfx)
		}
	}
	if sp := Cfg.GetSecretPath(); sp != "" && p == sp {
		return fmt.Errorf("path is reserved")
	}
	return nil
}

func FileOptionsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
}

func FileCreateHandler(w http.ResponseWriter, r *http.Request) {
	// #### CHECK IF AUTHENTICATED ####
	_, err := AuthSession(r)
	if err != nil {
		DumpResponse(w, "unauthorized", http.StatusUnauthorized, API_ERROR_BAD_AUTHENTICATION, nil)
		return
	}

	data_dir := Cfg.GetDataDir()
	user_id := 1

	file, fhead, err := r.FormFile("file")
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
		return
	}
	defer file.Close()

	name := utils.SanitizeUrlSegment(fhead.Filename)
	fname := utils.GenRandomHash()
	url_path := "/" + utils.GenRandomString(8) + "/" + name // TODO: make sure the generated folder is unique
	mime_type := fhead.Header.Get("content-type")           //r.Header.Get("content-type")
	if mime_type == "" {
		mime_type = "application/octet-stream"
	}
	log.Debug("upload: %s", mime_type)

	os.Mkdir(filepath.Join(data_dir, "files"), 0700)
	save_path := filepath.Join(data_dir, "files", fname)
	if err := SaveUploadedFile(file, fhead, save_path); err != nil {
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_SAVE_FAILED, nil)
		return
	}

	var fi os.FileInfo
	if fi, err = os.Stat(save_path); err != nil {
		os.Remove(save_path)
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_NOT_FOUND, nil)
		return
	}

	// Compute the blob hash before record creation so operators always see a
	// non-empty SHA256 on freshly uploaded files. Hash failure is non-fatal —
	// we'd rather the file appear with empty hash than reject the upload.
	sha, _ := utils.HashFile(save_path)

	o := &storage.DbFile{
		Uid:          user_id,
		Name:         name,
		Filename:     fname,
		FileSize:     fi.Size(),
		UrlPath:      url_path,
		RedirectPath: "",
		MimeType:     mime_type,
		SubMimeType:  mime_type,
		OrigMimeType: mime_type,
		CreateTime:   time.Now().Unix(),
		IsEnabled:    true,
		IsPaused:     false,
		RefSubFile:   0,
		SHA256:       sha,
	}

	f, err := storage.FileCreate(o)
	if err != nil {
		os.Remove(save_path)
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_DATABASE_FAILED, nil)
		return
	}
	DumpResponse(w, "ok", http.StatusOK, 0, f)
}

// FilePasteHandler creates a file whose blob is a JSON-supplied text body — the
// pastebin entry point. The created record is a regular DbFile, so it inherits
// the entire delivery pipeline (filters, password, expire, max-downloads, kill
// switch, log, notify, QR, rotate). When BurnAfterRead is set, the first
// successful download deletes the record + blob (enforced in core/http.go).
func FilePasteHandler(w http.ResponseWriter, r *http.Request) {
	if _, err := AuthSession(r); err != nil {
		DumpResponse(w, "unauthorized", http.StatusUnauthorized, API_ERROR_BAD_AUTHENTICATION, nil)
		return
	}

	limitBody(w, r, MaxPasteBody)
	type req struct {
		Name          string `json:"name"`
		MimeType      string `json:"mime_type"`
		Content       string `json:"content"`
		BurnAfterRead bool   `json:"burn_after_read"`
	}
	j := req{}
	if err := json.NewDecoder(r.Body).Decode(&j); err != nil {
		DumpResponse(w, err.Error(), http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
		return
	}
	if j.Name == "" {
		j.Name = "paste.txt"
	}
	if j.MimeType == "" {
		j.MimeType = "text/plain; charset=utf-8"
	}
	j.Name = utils.SanitizeUrlSegment(j.Name)

	data_dir := Cfg.GetDataDir()
	fname := utils.GenRandomHash()
	url_path := "/" + utils.GenRandomString(8) + "/" + j.Name

	os.Mkdir(filepath.Join(data_dir, "files"), 0700)
	save_path := filepath.Join(data_dir, "files", fname)
	if err := os.WriteFile(save_path, []byte(j.Content), 0600); err != nil {
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_SAVE_FAILED, nil)
		return
	}

	fi, err := os.Stat(save_path)
	if err != nil {
		os.Remove(save_path)
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_NOT_FOUND, nil)
		return
	}

	// Paste content is fully in memory — hash the source bytes directly rather
	// than the on-disk blob to avoid a needless re-read.
	sum := sha256.Sum256([]byte(j.Content))
	sha := hex.EncodeToString(sum[:])

	o := &storage.DbFile{
		Uid:           1,
		Name:          j.Name,
		Filename:      fname,
		FileSize:      fi.Size(),
		UrlPath:       url_path,
		RedirectPath:  "",
		MimeType:      j.MimeType,
		SubMimeType:   j.MimeType,
		OrigMimeType:  j.MimeType,
		CreateTime:    time.Now().Unix(),
		IsEnabled:     true,
		IsPaused:      false,
		RefSubFile:    0,
		BurnAfterRead: j.BurnAfterRead,
		SHA256:        sha,
	}
	f, err := storage.FileCreate(o)
	if err != nil {
		os.Remove(save_path)
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_DATABASE_FAILED, nil)
		return
	}
	DumpResponse(w, "ok", http.StatusOK, 0, f)
}

func FileListHandler(w http.ResponseWriter, r *http.Request) {
	// #### CHECK IF AUTHENTICATED ####
	_, err := AuthSession(r)
	if err != nil {
		DumpResponse(w, "unauthorized", http.StatusUnauthorized, API_ERROR_BAD_AUTHENTICATION, nil)
		return
	}

	files, err := storage.FileList()
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_DATABASE_FAILED, nil)
		return
	}
	type JsonFile struct {
		storage.DbFile
		SubFile     *storage.DbSubFile `json:"sub_file"`
		HasPassword bool               `json:"has_password"`
	}
	type Response struct {
		Uploads []*JsonFile `json:"uploads"`
	}
	resp := &Response{}

	for _, f := range files {
		jo := &JsonFile{
			DbFile:      f,
			HasPassword: storage.FileGetPasswordHash(f.ID) != "",
		}
		if f.RefSubFile > 0 {
			subf, err := storage.SubFileGet(f.RefSubFile)
			if err == nil {
				jo.SubFile = subf
			}
		}
		resp.Uploads = append(resp.Uploads, jo)
	}

	DumpResponse(w, "ok", http.StatusOK, 0, resp)
}

func FileDeleteHandler(w http.ResponseWriter, r *http.Request) {
	// #### CHECK IF AUTHENTICATED ####
	_, err := AuthSession(r)
	if err != nil {
		DumpResponse(w, "unauthorized", http.StatusUnauthorized, API_ERROR_BAD_AUTHENTICATION, nil)
		return
	}

	data_dir := Cfg.GetDataDir()
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
		return
	}

	f, err := storage.FileGet(id)
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusNotFound, API_ERROR_FILE_NOT_FOUND, nil)
		return
	}

	if f.RefSubFile > 0 {
		err = DeleteSubFile(f.RefSubFile)
		if err != nil {
			DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_DATABASE_FAILED, nil)
			return
		}
	}

	err = storage.FileDelete(id)
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_DATABASE_FAILED, nil)
		return
	}
	_ = storage.FilePasswordDelete(id)
	_ = storage.FilterDeleteForFile(id)
	save_path := filepath.Join(data_dir, "files", f.Filename)
	os.Remove(save_path)
	dropFileLock(id)

	DumpResponse(w, "ok", http.StatusOK, 0, nil)
}

// FileReplaceHandler swaps the on-disk content of an existing file while
// preserving its URL path, redirect path, mime, policy fields, password, and
// any per-file filter rules. The download counter is reset to 0 (operators
// want fresh counters when rotating a payload). Optionally accepts a new
// display name via form value "name".
// FileBulkHandler applies one action to a list of file ids in a single round
// trip. Saves the UI from making N requests when the operator multi-selects.
//
// Body: {"action":"enable|disable|pause|unpause|delete", "ids":[1,2,3]}
func FileBulkHandler(w http.ResponseWriter, r *http.Request) {
	if _, err := AuthSession(r); err != nil {
		DumpResponse(w, "unauthorized", http.StatusUnauthorized, API_ERROR_BAD_AUTHENTICATION, nil)
		return
	}

	limitBody(w, r, MaxJSONBody)
	type req struct {
		Action string `json:"action"`
		Ids    []int  `json:"ids"`
	}
	j := req{}
	if err := json.NewDecoder(r.Body).Decode(&j); err != nil {
		DumpResponse(w, err.Error(), http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
		return
	}
	if len(j.Ids) == 0 {
		DumpResponse(w, "ids is required", http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
		return
	}
	data_dir := Cfg.GetDataDir()
	ok, failed := 0, 0
	for _, id := range j.Ids {
		var err error
		switch j.Action {
		case "enable":
			_, err = storage.FileEnable(id, true)
		case "disable":
			_, err = storage.FileEnable(id, false)
		case "pause":
			_, err = storage.FilePause(id, true)
		case "unpause":
			_, err = storage.FilePause(id, false)
		case "delete":
			err = deleteFileFull(id, data_dir)
		default:
			DumpResponse(w, "unknown action", http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
			return
		}
		if err != nil {
			failed++
		} else {
			ok++
		}
	}
	DumpResponse(w, "ok", http.StatusOK, 0, map[string]int{"ok": ok, "failed": failed})
}

// deleteFileFull mirrors FileDeleteHandler's full cleanup so bulk delete and
// single delete behave identically (facade, password, per-file filters, blob).
func deleteFileFull(id int, dataDir string) error {
	f, err := storage.FileGet(id)
	if err != nil {
		return err
	}
	if f.RefSubFile > 0 {
		_ = DeleteSubFile(f.RefSubFile)
	}
	if err := storage.FileDelete(id); err != nil {
		return err
	}
	_ = storage.FilePasswordDelete(id)
	_ = storage.FilterDeleteForFile(id)
	os.Remove(filepath.Join(dataDir, "files", f.Filename))
	dropFileLock(id)
	return nil
}

// FileRotateUrlHandler generates a new random folder for the file's public
// UrlPath, keeping the filename portion intact so the URL still looks like the
// original payload. Everything else (blob, password, filters, counters, policy)
// is preserved — this is the "the link got burned" escape hatch.
func FileRotateUrlHandler(w http.ResponseWriter, r *http.Request) {
	if _, err := AuthSession(r); err != nil {
		DumpResponse(w, "unauthorized", http.StatusUnauthorized, API_ERROR_BAD_AUTHENTICATION, nil)
		return
	}

	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
		return
	}

	existing, err := storage.FileGet(id)
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusNotFound, API_ERROR_FILE_NOT_FOUND, nil)
		return
	}

	// Keep the trailing filename so the URL surface stays familiar; only the
	// random folder rotates. Fallback to the stored Name if UrlPath is empty
	// or malformed for any reason.
	tail := existing.Name
	if idx := strings.LastIndex(existing.UrlPath, "/"); idx >= 0 && idx+1 < len(existing.UrlPath) {
		tail = existing.UrlPath[idx+1:]
	}
	newPath := "/" + utils.GenRandomString(8) + "/" + tail

	if err := storage.FileRotateUrl(id, newPath); err != nil {
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_DATABASE_FAILED, nil)
		return
	}

	final, err := storage.FileGet(id)
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_DATABASE_FAILED, nil)
		return
	}
	DumpResponse(w, "ok", http.StatusOK, 0, final)
}

func FileReplaceHandler(w http.ResponseWriter, r *http.Request) {
	if _, err := AuthSession(r); err != nil {
		DumpResponse(w, "unauthorized", http.StatusUnauthorized, API_ERROR_BAD_AUTHENTICATION, nil)
		return
	}

	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
		return
	}

	existing, err := storage.FileGet(id)
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusNotFound, API_ERROR_FILE_NOT_FOUND, nil)
		return
	}

	file, fhead, err := r.FormFile("file")
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
		return
	}
	defer file.Close()

	data_dir := Cfg.GetDataDir()
	old_blob := filepath.Join(data_dir, "files", existing.Filename)
	new_fname := utils.GenRandomHash()
	new_blob := filepath.Join(data_dir, "files", new_fname)

	if err := SaveUploadedFile(file, fhead, new_blob); err != nil {
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_SAVE_FAILED, nil)
		return
	}
	fi, err := os.Stat(new_blob)
	if err != nil {
		os.Remove(new_blob)
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_NOT_FOUND, nil)
		return
	}

	// Patch only the fields that change on a replace: blob handle, size, mime
	// (from the new upload), display name if the client passed one, and the
	// download counter back to zero. Everything else stays.
	existing.Filename = new_fname
	existing.FileSize = fi.Size()
	if mt := fhead.Header.Get("content-type"); mt != "" {
		existing.MimeType = mt
		existing.OrigMimeType = mt
	}
	if newName := r.FormValue("name"); newName != "" {
		existing.Name = utils.SanitizeUrlSegment(newName)
	}

	// FileUpdate only touches a fixed subset of columns; the fields we change
	// on a replace (Filename, FileSize, OrigMimeType) need direct UpdateField
	// calls.
	if err := storage.FileReplaceBlob(id, new_fname, fi.Size(), existing.MimeType, existing.OrigMimeType, existing.Name); err != nil {
		os.Remove(new_blob)
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_DATABASE_FAILED, nil)
		return
	}
	_ = storage.FileResetDownloadCount(id)
	if sha, err := utils.HashFile(new_blob); err == nil {
		_ = storage.FileSetHash(id, sha)
	}

	// Old blob is now orphaned — drop it. Done last so a failure above doesn't
	// leave the file with no on-disk content.
	_ = os.Remove(old_blob)

	final, _ := storage.FileGet(id)
	DumpResponse(w, "ok", http.StatusOK, 0, final)
}

func FileUpdateHandler(w http.ResponseWriter, r *http.Request) {
	// #### CHECK IF AUTHENTICATED ####
	_, err := AuthSession(r)
	if err != nil {
		DumpResponse(w, "unauthorized", http.StatusUnauthorized, API_ERROR_BAD_AUTHENTICATION, nil)
		return
	}

	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
		return
	}

	existing, err := storage.FileGet(id)
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusNotFound, API_ERROR_FILE_DATABASE_FAILED, nil)
		return
	}

	// Decode into a struct that embeds DbFile plus the policy-related extras
	// (cleartext Password, ClearPassword) which never hit storage directly.
	// Baseline = existing record, so a partial PUT preserves omitted fields.
	type updateRequest struct {
		storage.DbFile
		Password      string `json:"password"`
		ClearPassword bool   `json:"clear_password"`
	}
	limitBody(w, r, MaxJSONBody)
	req := updateRequest{DbFile: *existing}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		DumpResponse(w, err.Error(), http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
		return
	}

	if req.UrlPath == "" {
		DumpResponse(w, "url_path is required", http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
		return
	}
	if req.UrlPath[0] != '/' {
		req.UrlPath = "/" + req.UrlPath
	}
	if len(req.RedirectPath) > 0 && req.RedirectPath[0] != '/' {
		req.RedirectPath = "/" + req.RedirectPath
	}
	if err := validateUserUrlPath(req.UrlPath); err != nil {
		DumpResponse(w, err.Error(), http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
		return
	}
	if req.RedirectPath != "" {
		if err := validateUserUrlPath(req.RedirectPath); err != nil {
			DumpResponse(w, err.Error(), http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
			return
		}
	}

	f, err := storage.FileUpdate(id, &req.DbFile)
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_DATABASE_FAILED, nil)
		return
	}

	// Policy fields are stored separately to keep FileUpdate's column-list
	// signature stable; the counter is owned by the download flow.
	// Normalise WrapAs to the supported set so a typoed client value doesn't
	// fall through to the serve path and bypass the switch (which would just
	// serve raw — but the operator would see an unfamiliar string in the UI).
	switch req.WrapAs {
	case "", "none", "zip":
	default:
		req.WrapAs = ""
	}
	if err := storage.FileUpdatePolicy(id, req.ExpireAt, req.MaxDownloads, req.NotifyMuted, req.BurnAfterRead, req.Watermark, req.Note, req.WrapAs); err != nil {
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_DATABASE_FAILED, nil)
		return
	}
	if req.ClearPassword {
		if err := storage.FileSetPasswordHash(id, ""); err != nil {
			DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_DATABASE_FAILED, nil)
			return
		}
	} else if req.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 10)
		if err != nil {
			DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_DATABASE_FAILED, nil)
			return
		}
		if err := storage.FileSetPasswordHash(id, string(hash)); err != nil {
			DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_DATABASE_FAILED, nil)
			return
		}
	}

	// Re-fetch so the response carries the persisted policy + has_password.
	final, err := storage.FileGet(id)
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_DATABASE_FAILED, nil)
		return
	}
	type updateResp struct {
		storage.DbFile
		HasPassword bool `json:"has_password"`
	}
	_ = f
	DumpResponse(w, "ok", http.StatusOK, 0, &updateResp{DbFile: *final, HasPassword: storage.FileGetPasswordHash(id) != ""})
}

func FileEnableHandler(w http.ResponseWriter, r *http.Request) {
	// #### CHECK IF AUTHENTICATED ####
	_, err := AuthSession(r)
	if err != nil {
		DumpResponse(w, "unauthorized", http.StatusUnauthorized, API_ERROR_BAD_AUTHENTICATION, nil)
		return
	}

	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
		return
	}
	f, err := storage.FileEnable(id, true)
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_DATABASE_FAILED, nil)
		return
	}
	DumpResponse(w, "ok", http.StatusOK, 0, f)
}

func FileDisableHandler(w http.ResponseWriter, r *http.Request) {
	// #### CHECK IF AUTHENTICATED ####
	_, err := AuthSession(r)
	if err != nil {
		DumpResponse(w, "unauthorized", http.StatusUnauthorized, API_ERROR_BAD_AUTHENTICATION, nil)
		return
	}

	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
		return
	}
	f, err := storage.FileEnable(id, false)
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_DATABASE_FAILED, nil)
		return
	}
	log.Debug("%v", f.IsEnabled)
	DumpResponse(w, "ok", http.StatusOK, 0, f)
}

func FilePauseHandler(w http.ResponseWriter, r *http.Request) {
	// #### CHECK IF AUTHENTICATED ####
	_, err := AuthSession(r)
	if err != nil {
		DumpResponse(w, "unauthorized", http.StatusUnauthorized, API_ERROR_BAD_AUTHENTICATION, nil)
		return
	}

	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
		return
	}
	f, err := storage.FilePause(id, true)
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_DATABASE_FAILED, nil)
		return
	}
	DumpResponse(w, "ok", http.StatusOK, 0, f)
}

func FileUnpauseHandler(w http.ResponseWriter, r *http.Request) {
	// #### CHECK IF AUTHENTICATED ####
	_, err := AuthSession(r)
	if err != nil {
		DumpResponse(w, "unauthorized", http.StatusUnauthorized, API_ERROR_BAD_AUTHENTICATION, nil)
		return
	}

	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
		return
	}
	f, err := storage.FilePause(id, false)
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_DATABASE_FAILED, nil)
		return
	}
	DumpResponse(w, "ok", http.StatusOK, 0, f)
}
