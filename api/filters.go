package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"

	"github.com/kgretzky/pwndrop/storage"
)

func FilterOptionsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
}

// FilterListHandler returns rules. Optional ?file_id=N narrows the result:
//   - omitted: all rules (both global and per-file)
//   - 0:       only global rules
//   - >0:      only rules tied to that file
func FilterListHandler(w http.ResponseWriter, r *http.Request) {
	if _, err := AuthSession(r); err != nil {
		DumpResponse(w, "unauthorized", http.StatusUnauthorized, API_ERROR_BAD_AUTHENTICATION, nil)
		return
	}

	fileId := -1
	if v := r.URL.Query().Get("file_id"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			fileId = n
		}
	}

	rules, err := storage.FilterList(fileId)
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_DATABASE_FAILED, nil)
		return
	}

	DumpResponse(w, "ok", http.StatusOK, 0, map[string]interface{}{"filters": rules})
}

func FilterCreateHandler(w http.ResponseWriter, r *http.Request) {
	if _, err := AuthSession(r); err != nil {
		DumpResponse(w, "unauthorized", http.StatusUnauthorized, API_ERROR_BAD_AUTHENTICATION, nil)
		return
	}

	limitBody(w, r, MaxJSONBody)
	o := storage.DbFilter{}
	if err := json.NewDecoder(r.Body).Decode(&o); err != nil {
		DumpResponse(w, err.Error(), http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
		return
	}
	if !validFilter(&o) {
		DumpResponse(w, "invalid filter", http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
		return
	}
	o.ID = 0
	o.CreateTime = time.Now().Unix()

	saved, err := storage.FilterCreate(&o)
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_DATABASE_FAILED, nil)
		return
	}
	DumpResponse(w, "ok", http.StatusOK, 0, saved)
}

func FilterUpdateHandler(w http.ResponseWriter, r *http.Request) {
	if _, err := AuthSession(r); err != nil {
		DumpResponse(w, "unauthorized", http.StatusUnauthorized, API_ERROR_BAD_AUTHENTICATION, nil)
		return
	}

	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
		return
	}

	existing, err := storage.FilterGet(id)
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusNotFound, API_ERROR_FILE_NOT_FOUND, nil)
		return
	}

	limitBody(w, r, MaxJSONBody)
	o := storage.DbFilter{}
	if err := json.NewDecoder(r.Body).Decode(&o); err != nil {
		DumpResponse(w, err.Error(), http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
		return
	}
	if !validFilter(&o) {
		DumpResponse(w, "invalid filter", http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
		return
	}
	// CreateTime, FileId and the running HitCount are immutable from the
	// client — HitCount is owned by the evaluator (FilterIncrementHits).
	o.CreateTime = existing.CreateTime
	o.FileId = existing.FileId
	o.HitCount = existing.HitCount

	saved, err := storage.FilterUpdate(id, &o)
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_DATABASE_FAILED, nil)
		return
	}
	DumpResponse(w, "ok", http.StatusOK, 0, saved)
}

func FilterDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if _, err := AuthSession(r); err != nil {
		DumpResponse(w, "unauthorized", http.StatusUnauthorized, API_ERROR_BAD_AUTHENTICATION, nil)
		return
	}

	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
		return
	}
	if err := storage.FilterDelete(id); err != nil {
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_DATABASE_FAILED, nil)
		return
	}
	DumpResponse(w, "ok", http.StatusOK, 0, nil)
}

func validFilter(o *storage.DbFilter) bool {
	switch o.MatchType {
	case "ip", "cidr", "country", "ua_regex":
	default:
		return false
	}
	switch o.Action {
	case "allow", "deny", "facade", "redirect":
	default:
		return false
	}
	if o.Pattern == "" {
		return false
	}
	return true
}
