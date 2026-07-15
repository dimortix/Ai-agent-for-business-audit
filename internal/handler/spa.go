package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// spaHandler раздаёт собранный фронтенд (web/dist) с fallback на index.html
// для клиентских роутов (/analytics, /advice, ...).
func spaHandler(webDir string) http.HandlerFunc {
	fileServer := http.FileServer(http.Dir(webDir))

	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeErr(w, http.StatusNotFound, "нет такого метода API")
			return
		}

		path := filepath.Join(webDir, filepath.Clean("/"+r.URL.Path))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			// sw.js и manifest не должны кэшироваться агрессивно
			if strings.HasSuffix(r.URL.Path, "sw.js") || strings.HasSuffix(r.URL.Path, ".webmanifest") {
				w.Header().Set("Cache-Control", "no-cache")
			}
			// стандартный mime-пакет не знает .webmanifest → отдал бы text/plain,
			// и Chrome не предлагает установку PWA
			if strings.HasSuffix(r.URL.Path, ".webmanifest") {
				w.Header().Set("Content-Type", "application/manifest+json")
			}
			fileServer.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, filepath.Join(webDir, "index.html"))
	}
}
