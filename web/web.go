// Package web holds the frontend files, compiled into the binary with go:embed.
package web

import (
	"embed"
	"net/http"
)

// files holds the frontend. The paths are relative to this directory, and
// go:embed cannot reach outside it, which is why this package lives here.
//
//go:embed index.html app.js
var files embed.FS

// Handler serves the embedded files. A request for "/" gets index.html.
func Handler() http.Handler {
	return http.FileServerFS(files)
}
