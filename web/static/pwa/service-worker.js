const CACHE_PREFIX = "tokenflow-pwa-";
const CACHE_NAME = `${CACHE_PREFIX}v15`;
const OFFLINE_URL = "/offline";
const PRECACHE_URLS = [
  "/manifest.webmanifest",
  "/admin/manifest.webmanifest",
  OFFLINE_URL,
  "/admin/static/tokenflow-logo.png",
  "/admin/static/pwa/icon-192.png",
  "/admin/static/pwa/icon-512.png",
  "/admin/static/pwa/icon-maskable-512.png",
  "/admin/static/pwa/register.js",
  "/admin/static/theme.js",
  "/admin/static/icons.svg",
  "/admin/static/css/tokens.css",
  "/admin/static/css/base.css",
  "/admin/static/css/components.css",
  "/admin/static/css/charts.css",
  "/admin/static/css/layout.css",
  "/admin/static/css/chat.css",
  "/admin/static/css/home.css",
  "/admin/static/core/api.js",
  "/admin/static/core/confirm.js",
  "/admin/static/core/dom.js",
  "/admin/static/core/format.js",
  "/admin/static/core/nav.js",
  "/admin/static/core/toast.js",
  "/admin/static/account/app.js",
  "/admin/static/home/app.js",
  "/admin/static/chat/app.js",
  "/admin/static/chat/highlight.bundle.js",
  "/admin/static/chat/html-preview.js",
  "/admin/static/chat/markdown.js",
  "/admin/static/chat/runtime.js"
];

self.addEventListener("install", (event) => {
  event.waitUntil(caches.open(CACHE_NAME).then((cache) => cache.addAll(PRECACHE_URLS)).then(() => self.skipWaiting()));
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches.keys()
      .then((keys) => Promise.all(keys.filter((key) => key.startsWith(CACHE_PREFIX) && key !== CACHE_NAME).map((key) => caches.delete(key))))
      .then(() => self.clients.claim())
  );
});

function isNetworkOnlyPath(pathname) {
  return pathname.startsWith("/account/api/") ||
    pathname.startsWith("/admin/api/") ||
    pathname.startsWith("/v1/") ||
    pathname.startsWith("/anthropic/v1/");
}

async function staleWhileRevalidate(request) {
  const cache = await caches.open(CACHE_NAME);
  const cached = await cache.match(request);
  const network = fetch(request).then((response) => {
    if (response.ok) cache.put(request, response.clone());
    return response;
  });
  return cached || network;
}

self.addEventListener("fetch", (event) => {
  const request = event.request;
  if (request.method !== "GET") return;
  const url = new URL(request.url);
  if (url.origin !== self.location.origin || isNetworkOnlyPath(url.pathname)) return;

  if (request.mode === "navigate") {
    event.respondWith(fetch(request).catch(() => caches.match(OFFLINE_URL)));
    return;
  }

  if (url.pathname.startsWith("/admin/static/") || url.pathname === "/manifest.webmanifest" || url.pathname === "/admin/manifest.webmanifest" || url.pathname === OFFLINE_URL) {
    event.respondWith(staleWhileRevalidate(request));
  }
});
