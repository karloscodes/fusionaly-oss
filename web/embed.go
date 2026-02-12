// Package web provides embedded static assets for production builds.
package web

import (
	"embed"
	"io/fs"
)

//go:embed dist/assets
var assetsFS embed.FS

// Assets returns the embedded static assets filesystem.
// Returns nil in development mode (assets served from disk for hot-reload).
// The returned fs.FS has the path prefix stripped (files at root, not dist/assets/).
func Assets() fs.FS {
	sub, err := fs.Sub(assetsFS, "dist/assets")
	if err != nil {
		panic(err)
	}
	return sub
}
