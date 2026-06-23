package api

import (
	"context"
	"net/http"
	"time"

	"github.com/kgretzky/pwndrop/core/updater"
	"github.com/kgretzky/pwndrop/log"
)

func UpdateOptionsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
}

func UpdateCheckHandler(w http.ResponseWriter, r *http.Request) {
	if _, err := AuthSession(r); err != nil {
		DumpResponse(w, "unauthorized", http.StatusUnauthorized, API_ERROR_BAD_AUTHENTICATION, nil)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	res, err := updater.Check(ctx)
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusBadGateway, API_ERROR_UPDATE_FAILED, nil)
		return
	}
	DumpResponse(w, "ok", http.StatusOK, 0, res)
}

func UpdateApplyHandler(w http.ResponseWriter, r *http.Request) {
	if _, err := AuthSession(r); err != nil {
		DumpResponse(w, "unauthorized", http.StatusUnauthorized, API_ERROR_BAD_AUTHENTICATION, nil)
		return
	}

	// Ack the request before we tear down the process. The actual swap +
	// re-exec runs in a goroutine so the client sees a clean 200 + body and
	// can start polling /version for the new build.
	DumpResponse(w, "update started", http.StatusOK, 0, map[string]string{"status": "started"})

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		if err := updater.Apply(ctx); err != nil {
			log.Error("updater: apply failed: %s", err)
		}
	}()
}
