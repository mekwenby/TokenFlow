package web

import (
	"encoding/json"
	"image/png"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPWAManifestAndIcons(t *testing.T) {
	raw, err := fs.ReadFile(Static, "static/pwa/manifest.webmanifest")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		ID              string `json:"id"`
		Name            string `json:"name"`
		ShortName       string `json:"short_name"`
		StartURL        string `json:"start_url"`
		Scope           string `json:"scope"`
		Display         string `json:"display"`
		ThemeColor      string `json:"theme_color"`
		BackgroundColor string `json:"background_color"`
		Icons           []struct {
			Source  string `json:"src"`
			Sizes   string `json:"sizes"`
			Type    string `json:"type"`
			Purpose string `json:"purpose"`
		} `json:"icons"`
		Shortcuts []struct {
			URL string `json:"url"`
		} `json:"shortcuts"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.ID != "/account/chat" || manifest.Name != "TokenFlow LLM Chat" || manifest.ShortName != "TokenFlow" || manifest.StartURL != "/account/chat" || manifest.Scope != "/" || manifest.Display != "standalone" {
		t.Fatalf("unexpected manifest identity: %#v", manifest)
	}
	if manifest.ThemeColor != "#101820" || manifest.BackgroundColor != "#101820" {
		t.Fatalf("unexpected manifest colors: %#v", manifest)
	}
	if len(manifest.Shortcuts) != 2 || manifest.Shortcuts[0].URL != "/account/chat" || manifest.Shortcuts[1].URL != "/account" {
		t.Fatalf("unexpected manifest shortcuts: %#v", manifest.Shortcuts)
	}

	expectedIcons := map[string]struct {
		size, purpose string
		width, height int
	}{
		"/admin/static/pwa/icon-192.png":          {size: "192x192", purpose: "any", width: 192, height: 192},
		"/admin/static/pwa/icon-512.png":          {size: "512x512", purpose: "any", width: 512, height: 512},
		"/admin/static/pwa/icon-maskable-512.png": {size: "512x512", purpose: "maskable", width: 512, height: 512},
	}
	if len(manifest.Icons) != len(expectedIcons) {
		t.Fatalf("unexpected manifest icons: %#v", manifest.Icons)
	}
	for _, icon := range manifest.Icons {
		expected, ok := expectedIcons[icon.Source]
		if !ok || icon.Sizes != expected.size || icon.Type != "image/png" || icon.Purpose != expected.purpose {
			t.Fatalf("unexpected icon declaration: %#v", icon)
		}
		file, err := Static.Open(strings.TrimPrefix(icon.Source, "/admin/"))
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := png.Decode(file)
		_ = file.Close()
		if err != nil {
			t.Fatalf("decode %s: %v", icon.Source, err)
		}
		bounds := decoded.Bounds()
		if bounds.Dx() != expected.width || bounds.Dy() != expected.height {
			t.Fatalf("unexpected dimensions for %s: %dx%d", icon.Source, bounds.Dx(), bounds.Dy())
		}
	}
}

func TestAdminPWAManifest(t *testing.T) {
	raw, err := fs.ReadFile(Static, "static/pwa/admin-manifest.webmanifest")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		StartURL  string `json:"start_url"`
		Scope     string `json:"scope"`
		Display   string `json:"display"`
		Shortcuts []struct {
			URL string `json:"url"`
		} `json:"shortcuts"`
		Icons []struct {
			Source  string `json:"src"`
			Purpose string `json:"purpose"`
		} `json:"icons"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.ID != "/admin/chat" || manifest.Name != "TokenFlow Admin" || manifest.StartURL != "/admin/chat" || manifest.Scope != "/" || manifest.Display != "standalone" {
		t.Fatalf("unexpected admin manifest identity: %#v", manifest)
	}
	if len(manifest.Shortcuts) != 2 || manifest.Shortcuts[0].URL != "/admin/chat" || manifest.Shortcuts[1].URL != "/admin" {
		t.Fatalf("unexpected admin manifest shortcuts: %#v", manifest.Shortcuts)
	}
	if len(manifest.Icons) != 3 || manifest.Icons[2].Purpose != "maskable" {
		t.Fatalf("unexpected admin manifest icons: %#v", manifest.Icons)
	}
}

func TestPWAAssetsAreEmbedded(t *testing.T) {
	for _, path := range []string{
		"static/tokenflow-logo.png",
		"static/pwa/manifest.webmanifest", "static/pwa/admin-manifest.webmanifest", "static/pwa/service-worker.js", "static/pwa/register.js",
		"static/pwa/offline.html", "static/pwa/icon-192.png", "static/pwa/icon-512.png",
		"static/pwa/icon-maskable-512.png",
	} {
		if _, err := fs.Stat(Static, path); err != nil {
			t.Errorf("embedded PWA asset %s: %v", path, err)
		}
	}
}

func TestPWAHandlersSetRequiredHeaders(t *testing.T) {
	tests := []struct {
		name, contentType, cacheControl, scope string
		handler                                http.HandlerFunc
	}{
		{name: "manifest", handler: Manifest, contentType: "application/manifest+json; charset=utf-8", cacheControl: "public, max-age=3600"},
		{name: "admin manifest", handler: AdminManifest, contentType: "application/manifest+json; charset=utf-8", cacheControl: "public, max-age=3600"},
		{name: "service worker", handler: ServiceWorker, contentType: "application/javascript; charset=utf-8", cacheControl: "no-cache", scope: "/"},
		{name: "offline", handler: Offline, contentType: "text/html; charset=utf-8", cacheControl: "no-cache"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			tc.handler(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
			if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Type") != tc.contentType || recorder.Header().Get("Cache-Control") != tc.cacheControl || recorder.Header().Get("Service-Worker-Allowed") != tc.scope {
				t.Fatalf("unexpected response: status=%d headers=%v", recorder.Code, recorder.Header())
			}
			if recorder.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatal("PWA response is missing nosniff")
			}
		})
	}
}

func TestServiceWorkerKeepsAuthenticatedTrafficNetworkOnly(t *testing.T) {
	raw, err := fs.ReadFile(Static, "static/pwa/service-worker.js")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	for _, expected := range []string{
		`request.method !== "GET"`, `pathname.startsWith("/account/api/")`,
		`pathname.startsWith("/admin/api/")`, `pathname.startsWith("/v1/")`,
		`pathname.startsWith("/anthropic/v1/")`, `request.mode === "navigate"`,
		`fetch(request).catch(() => caches.match(OFFLINE_URL))`, `self.skipWaiting()`, `self.clients.claim()`,
		`"/admin/static/icons.svg"`, `"/admin/static/css/charts.css"`,
		`"/admin/static/core/api.js"`, `"/admin/static/core/dom.js"`,
		`"/admin/static/core/format.js"`, `"/admin/static/core/toast.js"`,
		`"/admin/static/core/confirm.js"`, `"/admin/static/core/nav.js"`,
		"const CACHE_NAME = `${CACHE_PREFIX}v15`", `"/admin/manifest.webmanifest"`,
		`"/admin/static/tokenflow-logo.png"`,
		`"/admin/static/css/home.css"`, `"/admin/static/home/app.js"`,
		`"/admin/static/chat/highlight.bundle.js"`, `"/admin/static/chat/html-preview.js"`,
		`"/admin/static/chat/markdown.js"`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("service worker is missing %q", expected)
		}
	}
	for _, forbidden := range []string{`"/account/login"`, `"/account/chat"`, `"/admin/login"`} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("service worker should not cache authenticated HTML %q", forbidden)
		}
	}
}
