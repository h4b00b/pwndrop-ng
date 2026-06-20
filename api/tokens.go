package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"

	"github.com/kgretzky/pwndrop/storage"
	"github.com/kgretzky/pwndrop/utils"
)

// tokenInfo is the safe representation of an API token returned to the panel.
// The full secret is only ever returned once, at creation time.
type tokenInfo struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	TokenHint  string `json:"token_hint"`
	CreateTime int64  `json:"create_time"`
	LastUsed   int64  `json:"last_used"`
}

// maskToken returns a non-sensitive hint of a token (last 6 chars) for display.
func maskToken(token string) string {
	if len(token) <= 6 {
		return "******"
	}
	return "…" + token[len(token)-6:]
}

func TokenOptionsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE,OPTIONS")
}

func TokenListHandler(w http.ResponseWriter, r *http.Request) {
	if _, err := AuthSession(r); err != nil {
		DumpResponse(w, "unauthorized", http.StatusUnauthorized, API_ERROR_BAD_AUTHENTICATION, nil)
		return
	}

	tokens, err := storage.ApiTokenList()
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_DATABASE_FAILED, nil)
		return
	}

	out := []tokenInfo{}
	for _, t := range tokens {
		out = append(out, tokenInfo{
			ID:         t.ID,
			Name:       t.Name,
			TokenHint:  maskToken(t.Token),
			CreateTime: t.CreateTime,
			LastUsed:   t.LastUsed,
		})
	}

	DumpResponse(w, "ok", http.StatusOK, 0, map[string]interface{}{"tokens": out})
}

func TokenCreateHandler(w http.ResponseWriter, r *http.Request) {
	uid, err := AuthSession(r)
	if err != nil {
		DumpResponse(w, "unauthorized", http.StatusUnauthorized, API_ERROR_BAD_AUTHENTICATION, nil)
		return
	}

	limitBody(w, r, MaxJSONBody)
	type CreateTokenRequest struct {
		Name string `json:"name"`
	}
	j := CreateTokenRequest{}
	if err := json.NewDecoder(r.Body).Decode(&j); err != nil {
		DumpResponse(w, err.Error(), http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
		return
	}
	if j.Name == "" {
		j.Name = "unnamed"
	}

	token := utils.GenRandomHash()
	o := &storage.DbApiToken{
		Uid:        uid,
		Name:       j.Name,
		Token:      token,
		CreateTime: time.Now().Unix(),
		LastUsed:   0,
	}
	if _, err := storage.ApiTokenCreate(o); err != nil {
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_DATABASE_FAILED, nil)
		return
	}

	// the full token is returned only here, once
	DumpResponse(w, "ok", http.StatusOK, 0, map[string]interface{}{
		"id":          o.ID,
		"name":        o.Name,
		"token":       o.Token,
		"create_time": o.CreateTime,
	})
}

func TokenDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if _, err := AuthSession(r); err != nil {
		DumpResponse(w, "unauthorized", http.StatusUnauthorized, API_ERROR_BAD_AUTHENTICATION, nil)
		return
	}

	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
		return
	}

	if err := storage.ApiTokenDelete(id); err != nil {
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_DATABASE_FAILED, nil)
		return
	}

	DumpResponse(w, "ok", http.StatusOK, 0, nil)
}
