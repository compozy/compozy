const CLIENT_ID_STORAGE_KEY = "compozy.window-manager.client-id";

function randomClientId(): string {
  const cryptoApi = globalThis.crypto;
  if (typeof cryptoApi?.randomUUID === "function") {
    return `web-${cryptoApi.randomUUID()}`;
  }
  if (typeof cryptoApi?.getRandomValues === "function") {
    const bytes = cryptoApi.getRandomValues(new Uint8Array(16));
    return `web-${Array.from(bytes, value => value.toString(16).padStart(2, "0")).join("")}`;
  }
  return `web-${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

export function stableWindowManagerClientId(): string {
  if (typeof window === "undefined") return "web-server-render";
  const created = randomClientId();
  try {
    const existing = window.localStorage.getItem(CLIENT_ID_STORAGE_KEY)?.trim();
    if (existing) return existing;
    window.localStorage.setItem(CLIENT_ID_STORAGE_KEY, created);
  } catch {
    return created;
  }
  return created;
}
