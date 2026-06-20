package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"

	"github.com/kgretzky/pwndrop/filter"
)

// RuleTestHandler simulates one download request through the filter chain and
// returns the decision (action + matched rule). Useful for "would this rule
// catch the Burp scanner?" without having to spin up a real client.
//
// Body:
//
//	{"file_id":N, "remote_ip":"1.2.3.4", "user_agent":"curl/8"}
//
// Response:
//
//	{"action":"deny","rule_id":3,"pattern":"...","match_type":"cidr",
//	 "log_tag":"filter-deny:cidr:..."}
func RuleTestHandler(w http.ResponseWriter, r *http.Request) {
	if _, err := AuthSession(r); err != nil {
		DumpResponse(w, "unauthorized", http.StatusUnauthorized, API_ERROR_BAD_AUTHENTICATION, nil)
		return
	}

	type req struct {
		FileId    int    `json:"file_id"`
		RemoteIp  string `json:"remote_ip"`
		UserAgent string `json:"user_agent"`
	}
	limitBody(w, r, MaxJSONBody)
	j := req{}
	if err := json.NewDecoder(r.Body).Decode(&j); err != nil {
		DumpResponse(w, err.Error(), http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
		return
	}
	if j.RemoteIp == "" {
		DumpResponse(w, "remote_ip is required", http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
		return
	}

	// Fake a minimal *http.Request so EvaluateFilters works unchanged — we only
	// touch fields the evaluator actually reads (User-Agent header).
	fake := httptest.NewRequest("GET", "/", nil)
	if j.UserAgent != "" {
		fake.Header.Set("User-Agent", j.UserAgent)
	}

	dec, on := filter.Evaluate(fake, j.FileId, j.RemoteIp)
	resp := map[string]interface{}{
		"filters_enabled": on,
		"action":          string(dec.Action),
		"rule_id":         dec.RuleId,
		"pattern":         dec.Pattern,
		"match_type":      dec.Type,
		"log_tag":         dec.LogTag(),
	}
	// One-line human-readable summary for the UI.
	if !on {
		resp["summary"] = "filters disabled — request would pass to payload"
	} else if dec.RuleId == 0 {
		resp["summary"] = "no rule matched — default action: " + string(dec.Action)
	} else {
		resp["summary"] = "rule #" + strconv.Itoa(dec.RuleId) + " matched (" + dec.Type + "=" + dec.Pattern + ") → " + string(dec.Action)
	}

	DumpResponse(w, "ok", http.StatusOK, 0, resp)
}
