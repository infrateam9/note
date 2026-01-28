package main

import (
	"bytes"
	"embed"
	"net/http"
	"time"
)

//go:embed favicon.ico
var embeddedAssets embed.FS

func serveFavicon(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	b, err := embeddedAssets.ReadFile("favicon.ico")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/x-icon")
	http.ServeContent(w, r, "favicon.ico", time.Time{}, bytes.NewReader(b))
}
