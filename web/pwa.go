package web

import (
	"io/fs"
	"net/http"
)

func Manifest(w http.ResponseWriter, r *http.Request) {
	servePWAAsset(w, "static/pwa/manifest.webmanifest", "application/manifest+json; charset=utf-8", "public, max-age=3600")
}

func AdminManifest(w http.ResponseWriter, r *http.Request) {
	servePWAAsset(w, "static/pwa/admin-manifest.webmanifest", "application/manifest+json; charset=utf-8", "public, max-age=3600")
}

func ServiceWorker(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Service-Worker-Allowed", "/")
	servePWAAsset(w, "static/pwa/service-worker.js", "application/javascript; charset=utf-8", "no-cache")
}

func Offline(w http.ResponseWriter, r *http.Request) {
	servePWAAsset(w, "static/pwa/offline.html", "text/html; charset=utf-8", "no-cache")
}

func servePWAAsset(w http.ResponseWriter, path, contentType, cacheControl string) {
	raw, err := fs.ReadFile(Static, path)
	if err != nil {
		http.Error(w, "PWA asset unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", cacheControl)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(raw)
}
