// Package web provides embedded static assets for production builds.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// Assets returns the hashed assets (JS, CSS) served under /assets.
func Assets() fs.FS {
	sub, err := fs.Sub(distFS, "dist/assets")
	if err != nil {
		panic(err)
	}
	return sub
}

// PublicFiles returns root-level public files (favicon.svg, robots.txt) served at /.
func PublicFiles() fs.FS {
	return &rootFilesOnly{distFS: distFS}
}

// ManifestJSON returns the embedded Vite manifest.json contents.
func ManifestJSON() []byte {
	data, err := distFS.ReadFile("dist/.vite/manifest.json")
	if err != nil {
		return nil
	}
	return data
}

// rootFilesOnly exposes only the top-level files from dist/ (not subdirectories).
type rootFilesOnly struct {
	distFS embed.FS
}

func (r *rootFilesOnly) Open(name string) (fs.File, error) {
	return r.distFS.Open("dist/" + name)
}

func (r *rootFilesOnly) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, err := fs.ReadDir(r.distFS, "dist")
	if err != nil {
		return nil, err
	}
	var files []fs.DirEntry
	for _, e := range entries {
		if !e.IsDir() {
			files = append(files, e)
		}
	}
	return files, nil
}
