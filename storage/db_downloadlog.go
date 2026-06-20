package storage

// DbDownloadLog records a single download (or redirect) of a hosted file.
// Status values: "ok" (file served), "redirect" (handed off to RedirectPath),
// "paused-facade" (served the facade content).
type DbDownloadLog struct {
	ID         int    `json:"id" storm:"id,increment"`
	FileId     int    `json:"file_id" storm:"index"`
	FileName   string `json:"file_name"`
	UrlPath    string `json:"url_path"`
	RemoteIp   string `json:"remote_ip" storm:"index"`
	UserAgent  string `json:"user_agent"`
	Referer    string `json:"referer"`
	Status     string `json:"status"`
	Timestamp  int64  `json:"timestamp" storm:"index"`
}

func DownloadLogCreate(o *DbDownloadLog) (*DbDownloadLog, error) {
	if err := db.Save(o); err != nil {
		return nil, err
	}
	return o, nil
}

// DownloadLogList returns the most recent entries first. Limit <= 0 means no
// limit. The DB is small (one row per download), so an in-memory reverse is
// fine and avoids a custom index.
func DownloadLogList(limit int) ([]DbDownloadLog, error) {
	var os []DbDownloadLog
	query := db.Select().OrderBy("Timestamp").Reverse()
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&os); err != nil {
		if err.Error() == "not found" {
			return []DbDownloadLog{}, nil
		}
		return nil, err
	}
	return os, nil
}

// DownloadLogDelete removes a single entry by ID. Used by the cleanup
// goroutine to trim old entries one-by-one.
func DownloadLogDelete(id int) error {
	return db.DeleteStruct(&DbDownloadLog{ID: id})
}

func DownloadLogClear() error {
	var os []DbDownloadLog
	if err := db.All(&os); err != nil {
		return err
	}
	for _, o := range os {
		_ = db.DeleteStruct(&o)
	}
	return nil
}
