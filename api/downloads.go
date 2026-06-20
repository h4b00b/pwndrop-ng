package api

import (
	"net/http"
	"strconv"

	"github.com/kgretzky/pwndrop/storage"
)

const downloadListDefaultLimit = 200

func DownloadOptionsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Methods", "GET,DELETE,OPTIONS")
}

func DownloadListHandler(w http.ResponseWriter, r *http.Request) {
	if _, err := AuthSession(r); err != nil {
		DumpResponse(w, "unauthorized", http.StatusUnauthorized, API_ERROR_BAD_AUTHENTICATION, nil)
		return
	}

	limit := downloadListDefaultLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 5000 {
			limit = n
		}
	}

	logs, err := storage.DownloadLogList(limit)
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_DATABASE_FAILED, nil)
		return
	}

	DumpResponse(w, "ok", http.StatusOK, 0, map[string]interface{}{"downloads": logs})
}

func DownloadClearHandler(w http.ResponseWriter, r *http.Request) {
	if _, err := AuthSession(r); err != nil {
		DumpResponse(w, "unauthorized", http.StatusUnauthorized, API_ERROR_BAD_AUTHENTICATION, nil)
		return
	}
	if err := storage.DownloadLogClear(); err != nil {
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_DATABASE_FAILED, nil)
		return
	}
	DumpResponse(w, "ok", http.StatusOK, 0, nil)
}
