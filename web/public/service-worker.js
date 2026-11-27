self.addEventListener("install", event => event.waitUntil(self.skipWaiting()));
self.addEventListener("activate", event => event.waitUntil(self.clients.claim()));

const appAsset = /^\/assets\/[^/]+-[A-Za-z0-9_-]{8}\.(js|css)$/;

self.addEventListener("fetch", event => {
  const { request } = event;
  const url = new URL(request.url);
  if (
    request.method === "GET" &&
    request.cache === "default" &&
    url.origin === self.location.origin &&
    url.search === "" &&
    appAsset.test(url.pathname)
  ) {
    event.respondWith(fetch(request, { cache: "force-cache" }));
  }
});
