if (location.protocol === "https:" && "serviceWorker" in navigator) {
  fetch("/remote/v1/browser/status", { cache: "no-store" })
    .then(response => {
      if (response.ok && response.headers.get("content-type")?.startsWith("application/json")) {
        return navigator.serviceWorker.register("/service-worker.js");
      }
    })
    .catch(error => console.debug("Could not register the hosted app service worker", error));
}
