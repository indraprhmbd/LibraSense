package webui

import (
	"embed"
	"io/fs"
)

//go:embed index.html app.css app.js vendor
var content embed.FS

func FS() (fs.FS, error) {
	return fs.Sub(content, ".")
}
