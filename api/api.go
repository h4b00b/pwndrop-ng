package api

import (
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"os"

	"github.com/kgretzky/pwndrop/config"
)

type ApiResponse struct {
	ErrorCode int         `json:"error_code"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
}

// Body size caps for handlers that read a request body. Control-plane JSON
// payloads are small (a few KB at most) so 1 MiB is plenty; the paste endpoint
// receives the entire blob inline and gets its own larger cap.
const (
	MaxJSONBody  int64 = 1 << 20      // 1 MiB
	MaxPasteBody int64 = 50 << 20     // 50 MiB
)

// limitBody caps the request body to max bytes. Reads past the cap return an
// error from the underlying json.Decoder / io.Reader, which surfaces to the
// caller as a 400 / 413. Cheap belt-and-braces against memory exhaustion from
// an authenticated client (compromised token, runaway script).
func limitBody(w http.ResponseWriter, r *http.Request, max int64) {
	r.Body = http.MaxBytesReader(w, r.Body, max)
}

var Cfg *config.Config = nil

func SaveUploadedFile(file multipart.File, fhead *multipart.FileHeader, save_path string) error {
	// O_TRUNC is belt-and-braces: the random Filename means collisions are
	// effectively impossible, but if we ever DO collide we'd rather overwrite
	// cleanly than splice a hybrid blob.
	f, err := os.OpenFile(save_path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, file)
	if err != nil {
		return err
	}
	return nil
}

func DumpResponse(w http.ResponseWriter, message string, http_status int, error_code int, o interface{}) {
	resp := &ApiResponse{
		ErrorCode: error_code,
		Message:   message,
		Data:      o,
	}

	d, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, "corrupted response", http.StatusInternalServerError)
		return
	}
	// Headers MUST be set before WriteHeader — the old order silently dropped
	// Content-Type because net/http freezes the header map at WriteHeader.
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http_status)
	w.Write(d)
}

func SetConfig(cfg *config.Config) {
	Cfg = cfg
}
