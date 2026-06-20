package main

import (
	"embed"
	"io/fs"
)

// embeddedWeb contains the built admin panel (Vue 3 + Vite output), compiled
// into the binary so that pwndrop can be deployed as a single file with no
// companion "admin" dir. Run `make build` (or `npm run build` in frontend/)
// before `go build` so that frontend/dist exists.
//
//go:embed all:frontend/dist
var embeddedWeb embed.FS

// webFS returns the embedded web content rooted at the frontend build output.
func webFS() fs.FS {
	sub, err := fs.Sub(embeddedWeb, "frontend/dist")
	if err != nil {
		panic(err)
	}
	return sub
}
