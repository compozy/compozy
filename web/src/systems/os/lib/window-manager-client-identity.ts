export const WINDOW_MANAGER_CLIENT_ID_STORAGE_KEY = "compozy.window-manager.client-id";
export const WINDOW_MANAGER_CLIENT_ID_MEMORY_KEY = "compozy.window-manager.client-id.memory";

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

function rememberedClientId(): string | null {
  const cached = Reflect.get(globalThis, WINDOW_MANAGER_CLIENT_ID_MEMORY_KEY);
  return typeof cached === "string" && cached.trim() !== "" ? cached : null;
}

function rememberClientId(clientId: string): string {
  Reflect.set(globalThis, WINDOW_MANAGER_CLIENT_ID_MEMORY_KEY, clientId);
  return clientId;
}

export function stableWindowManagerClientId(): string {
  if (typeof window === "undefined") return "web-server-render";
  const cached = rememberedClientId();
  if (cached) return cached;
  try {
    const existing = window.sessionStorage.getItem(WINDOW_MANAGER_CLIENT_ID_STORAGE_KEY)?.trim();
    if (existing) return rememberClientId(existing);
    const created = randomClientId();
    window.sessionStorage.setItem(WINDOW_MANAGER_CLIENT_ID_STORAGE_KEY, created);
    return rememberClientId(created);
  } catch {
    return rememberClientId(randomClientId());
  }
}
