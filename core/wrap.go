package core

import (
	"archive/zip"
	"fmt"
	"io"
	"strings"
	"time"
)

// wrapKind describes a container repacker. Add new kinds (iso, lnk, …) by
// appending a case here and another entry in the switch in serveWrapped — the
// rest of the serve pipeline doesn't need to change.
type wrapKind struct {
	contentType string
	extension   string
}

// wrapKinds is the registry of supported WrapAs values. "" / "none" are
// handled by the caller (no wrap).
var wrapKinds = map[string]wrapKind{
	"zip": {contentType: "application/zip", extension: ".zip"},
}

// wrapInfo returns the kind metadata for a WrapAs value, plus ok=false when
// the value is unknown or empty. The serve path uses ok to decide whether to
// take the wrapped or raw branch.
func wrapInfo(wrapAs string) (wrapKind, bool) {
	k, ok := wrapKinds[strings.ToLower(wrapAs)]
	return k, ok
}

// wrappedFilename derives the filename the target will see (Content-Disposition
// and in-container entry name) from the original file name and the wrap kind.
// Preserves the original extension inside the container — a ".exe" payload
// becomes ".exe" inside a ".zip" so the target's mental model of "this is the
// file" stays intact after they extract.
func wrappedFilename(origName string, k wrapKind) string {
	if origName == "" {
		origName = "file"
	}
	return origName + k.extension
}

// serveWrapped streams the wrapped container directly to w. Returns the
// number of bytes written to the underlying body so the caller can decide
// what counts as a "delivered" download (>0 + no error). No Content-Length
// is set — Go's net/http falls back to chunked transfer-encoding for these.
func serveWrapped(w io.Writer, kind string, inner io.Reader, innerName string, watermarkSuffix []byte) (int64, error) {
	switch strings.ToLower(kind) {
	case "zip":
		zw := zip.NewWriter(w)
		fh := &zip.FileHeader{
			Name:     innerName,
			Method:   zip.Store, // no compression — payload may already be packed/encrypted; STORE keeps CPU off the box
			Modified: time.Now(),
		}
		fw, err := zw.CreateHeader(fh)
		if err != nil {
			return 0, err
		}
		n, err := io.Copy(fw, inner)
		if err != nil {
			return n, err
		}
		// Watermark goes INSIDE the container so the operator can still grep
		// the extracted file for the tag — outside the container it'd be in
		// the ZIP trailer where most readers ignore it.
		if len(watermarkSuffix) > 0 {
			if _, werr := fw.Write(watermarkSuffix); werr != nil {
				return n, werr
			}
			n += int64(len(watermarkSuffix))
		}
		if err := zw.Close(); err != nil {
			return n, err
		}
		return n, nil
	}
	return 0, fmt.Errorf("unsupported wrap kind: %s", kind)
}
