package handler

import (
	_ "embed"
	"net/http"
)

//go:embed web/index.html
var indexHTML []byte

func serveUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(indexHTML)
}
