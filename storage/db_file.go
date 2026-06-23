package storage

import (
	"github.com/kgretzky/pwndrop/log"
)

type DbFile struct {
	ID           int    `json:"id" storm:"id,increment"`
	Uid          int    `json:"uid" storm:"index"`
	Name         string `json:"name"`
	Filename     string `json:"fname"`
	FileSize     int64  `json:"fsize"`
	UrlPath      string `json:"url_path" storm:"unique"`
	MimeType     string `json:"mime_type"`
	OrigMimeType string `json:"orig_mime_type"`
	CreateTime   int64  `json:"create_time" storm:"index"`
	IsEnabled    bool   `json:"is_enabled"`
	IsPaused     bool   `json:"is_paused"`
	RedirectPath string `json:"redirect_path" storm:"unique"`
	SubName      string `json:"sub_name"`
	SubMimeType  string `json:"sub_mime_type"`
	RefSubFile   int    `json:"ref_sub_file"`

	// Per-file delivery policy. All optional.
	// ExpireAt: unix ts after which downloads are blocked (0 = never expires).
	// MaxDownloads: stop serving after this many successful downloads (0 = unlimited).
	// DownloadCount: server-side counter, incremented on every served download.
	// PasswordHash: bcrypt hash; when non-empty, downloader must pass Basic auth (any username, password = the cleartext).
	ExpireAt      int64 `json:"expire_at"`
	MaxDownloads  int   `json:"max_downloads"`
	DownloadCount int   `json:"download_count"`

	// NotifyOnAccess: per-file gate on outbound notifications. Older rows
	// default to false at zero-value, so init the field at upload time to
	// match the user expectation that "new files notify by default". We use
	// a NotifyMuted inverted flag instead so the zero-value (false) matches
	// "notify".
	NotifyMuted bool `json:"notify_muted"`

	// BurnAfterRead: when true, the first successful download deletes the
	// record + blob (handled in core/http.go via core.BurnFile). The paste
	// modal sets this; it's also surfaced in the file edit modal for uploads.
	BurnAfterRead bool `json:"burn_after_read"`

	// SHA256: hex-encoded sha256 of the stored blob. Computed at upload /
	// paste / replace / chunked-complete time. Exposed to operators (UI + API
	// response) and to downloaders via the X-Content-SHA256 header and the
	// RFC-3230 Digest header on the serve path, so a target can verify the
	// payload they fetched matches what the operator uploaded.
	SHA256 string `json:"sha256"`

	// Note: free-text operator memo for this file (campaign tag, target,
	// reminder of what payload this is). Surfaced in the edit modal only —
	// never sent on the download response or to any third-party sink.
	Note string `json:"note"`

	// Watermark: when true, every served body has a unique tag appended in
	// the form "\x00PWN:<32hex>\n". The tag goes into the download log so a
	// leaked sample (grep "PWN:") maps back to IP/UA/timestamp. Side-effects:
	// each download has a different content hash, so the Digest header is
	// suppressed and X-Content-Watermarked:true is sent instead. Range/resume
	// is auto-disabled (modified bytes break partial fetches). Tolerated by
	// PE/ELF (overlay/trailer) and most ZIP-based formats; documented as
	// unsafe for strict-parser formats (PDF, ISO9660).
	Watermark bool `json:"watermark"`

	// WrapAs requests on-the-fly container repackaging at serve time. Values:
	//   "" / "none": serve raw blob (default).
	//   "zip":       wrap in a single-entry ZIP (no compression) — bypasses
	//                mail filters that block bare .exe; the in-zip filename is
	//                preserved so the target sees the original payload name.
	// Reserved for future iteration: "iso" (MotW bypass on Windows — needs a
	// minimal ISO9660 writer; staged for 9f).
	// Side-effects: Content-Type switches to the wrap mime, X-Content-Wrapped
	// is emitted, Digest is suppressed (wrapped bytes differ from stored blob),
	// Range/resume is auto-disabled.
	WrapAs string `json:"wrap_as"`
}

func FileCreate(o *DbFile) (*DbFile, error) {
	err := db.Save(o)
	if err != nil {
		return nil, err
	}
	log.Debug("file id: %d", o.ID)
	return o, nil
}

func FileList() ([]DbFile, error) {
	var dbos []DbFile

	err := db.All(&dbos)
	if err != nil {
		return nil, err
	}
	return dbos, nil
}

func FileGet(id int) (*DbFile, error) {
	var o DbFile
	err := db.One("ID", id, &o)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func FileGetByUrl(url string) (*DbFile, error) {
	var o DbFile
	err := db.One("UrlPath", url, &o)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func FileGetByRedirectUrl(url string) (*DbFile, error) {
	var o DbFile
	err := db.One("RedirectPath", url, &o)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func FileDirExists(url string) bool {
	var o []DbFile
	if url == "" {
		return false
	}
	if url[len(url)-1] != '/' {
		url += "/"
	}
	err := db.Prefix("UrlPath", url, &o)
	if err != nil {
		return false
	}
	return true
}

func FileDelete(id int) error {
	f := &DbFile{
		ID: id,
	}
	err := db.DeleteStruct(f)
	if err != nil {
		return err
	}
	return nil
}

// FileUpdate applies the user-editable fields (display name, public URL,
// mime, redirect path, facade mime) to the file row. Blob handle (Filename),
// size, sub-file reference, counters and policy fields are intentionally NOT
// taken from the client here — they have dedicated paths (FileReplaceBlob,
// FileUpdatePolicy, FileResetSubFile, SubFile handlers). This keeps an
// authenticated client (or a compromised token) from re-pointing a public URL
// at another file's on-disk blob via the generic /files/{id} PUT.
func FileUpdate(id int, o *DbFile) (*DbFile, error) {
	if err := db.UpdateField(&DbFile{ID: id}, "Name", o.Name); err != nil {
		return nil, err
	}
	if err := db.UpdateField(&DbFile{ID: id}, "UrlPath", o.UrlPath); err != nil {
		return nil, err
	}
	if err := db.UpdateField(&DbFile{ID: id}, "MimeType", o.MimeType); err != nil {
		return nil, err
	}
	if err := db.UpdateField(&DbFile{ID: id}, "RedirectPath", o.RedirectPath); err != nil {
		return nil, err
	}
	if err := db.UpdateField(&DbFile{ID: id}, "SubMimeType", o.SubMimeType); err != nil {
		return nil, err
	}
	return FileGet(id)
}

// FileSetSubFile is used by the sub-file create path to wire the parent record
// to its facade. Kept separate from FileUpdate so the generic edit endpoint
// cannot point an arbitrary file at someone else's sub-file blob.
func FileSetSubFile(id int, refSubFile int, subName string) error {
	if err := db.UpdateField(&DbFile{ID: id}, "RefSubFile", refSubFile); err != nil {
		return err
	}
	if err := db.UpdateField(&DbFile{ID: id}, "SubName", subName); err != nil {
		return err
	}
	return nil
}

// FileUpdatePolicy sets the per-file expiry/quota fields. Counter is not
// touched here — it's owned by the download flow via FileIncrementDownloads.
func FileUpdatePolicy(id int, expireAt int64, maxDownloads int, notifyMuted, burnAfterRead, watermark bool, note, wrapAs string) error {
	if err := db.UpdateField(&DbFile{ID: id}, "ExpireAt", expireAt); err != nil {
		return err
	}
	if err := db.UpdateField(&DbFile{ID: id}, "MaxDownloads", maxDownloads); err != nil {
		return err
	}
	if err := db.UpdateField(&DbFile{ID: id}, "NotifyMuted", notifyMuted); err != nil {
		return err
	}
	if err := db.UpdateField(&DbFile{ID: id}, "BurnAfterRead", burnAfterRead); err != nil {
		return err
	}
	if err := db.UpdateField(&DbFile{ID: id}, "Watermark", watermark); err != nil {
		return err
	}
	if err := db.UpdateField(&DbFile{ID: id}, "Note", note); err != nil {
		return err
	}
	if err := db.UpdateField(&DbFile{ID: id}, "WrapAs", wrapAs); err != nil {
		return err
	}
	return nil
}

// FileSetPasswordHash stores a new bcrypt hash for the file. Pass "" to clear
// the password (deletes the row). The hash lives in DbFilePassword, a separate
// bucket, so DbFile responses never have to filter it out.
func FileSetPasswordHash(id int, hash string) error {
	if hash == "" {
		return FilePasswordDelete(id)
	}
	o := &DbFilePassword{ID: id, Hash: hash}
	return db.Save(o)
}

// FileGetPasswordHash returns the bcrypt hash, or "" when no password is set.
func FileGetPasswordHash(id int) string {
	o, err := FilePasswordGet(id)
	if err != nil {
		return ""
	}
	return o.Hash
}

// FileSetHash stores the hex sha256 for the file's stored blob. Called from
// the upload / paste / replace / chunked-complete paths after the blob has
// been written to disk. Empty string is allowed (legacy rows pre-feature have
// no hash yet).
func FileSetHash(id int, hex string) error {
	return db.UpdateField(&DbFile{ID: id}, "SHA256", hex)
}

// FileResetDownloadCount sets DownloadCount back to 0 — used by the replace
// endpoint so a rotated payload starts fresh against its MaxDownloads quota.
func FileResetDownloadCount(id int) error {
	return db.UpdateField(&DbFile{ID: id}, "DownloadCount", 0)
}

// FileReplaceBlob swaps the file's on-disk handle (Filename) and the cached
// metadata that goes with it. Used by the replace endpoint — the other policy
// fields are left as-is so the URL / password / filters carry over.
func FileReplaceBlob(id int, filename string, size int64, mimeType, origMimeType, name string) error {
	if err := db.UpdateField(&DbFile{ID: id}, "Filename", filename); err != nil {
		return err
	}
	if err := db.UpdateField(&DbFile{ID: id}, "FileSize", size); err != nil {
		return err
	}
	if err := db.UpdateField(&DbFile{ID: id}, "MimeType", mimeType); err != nil {
		return err
	}
	if err := db.UpdateField(&DbFile{ID: id}, "OrigMimeType", origMimeType); err != nil {
		return err
	}
	if name != "" {
		if err := db.UpdateField(&DbFile{ID: id}, "Name", name); err != nil {
			return err
		}
	}
	return nil
}

// FileRotateUrl swaps the public UrlPath. The new path goes through storm's
// unique index, so collisions surface as an error and the old URL stays put.
func FileRotateUrl(id int, newPath string) error {
	return db.UpdateField(&DbFile{ID: id}, "UrlPath", newPath)
}

// FileIncrementDownloads bumps DownloadCount by 1 and returns the new value.
// Uses read-modify-write — the download path is not high-concurrency on a
// red-team delivery target, and bbolt serializes writes anyway.
func FileIncrementDownloads(id int) (int, error) {
	f, err := FileGet(id)
	if err != nil {
		return 0, err
	}
	n := f.DownloadCount + 1
	if err := db.UpdateField(&DbFile{ID: id}, "DownloadCount", n); err != nil {
		return 0, err
	}
	return n, nil
}

func FileResetSubFile(id int) (*DbFile, error) {
	if err := db.UpdateField(&DbFile{ID: id}, "RefSubFile", 0); err != nil {
		return nil, err
	}
	o, err := FileGet(id)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func FileEnable(id int, enable bool) (*DbFile, error) {
	if err := db.UpdateField(&DbFile{ID: id}, "IsEnabled", enable); err != nil {
		return nil, err
	}
	o, err := FileGet(id)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func FilePause(id int, pause bool) (*DbFile, error) {
	if err := db.UpdateField(&DbFile{ID: id}, "IsPaused", pause); err != nil {
		return nil, err
	}
	o, err := FileGet(id)
	if err != nil {
		return nil, err
	}
	return o, nil
}
