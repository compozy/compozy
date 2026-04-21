export const WORKSPACE_STORAGE_KEY = "compozy.web.active-workspace";

function safeStorage(): Storage | null {
  try {
    if (typeof window === "undefined") {
      return null;
    }
    const storage = window.sessionStorage;
    return storage ?? null;
  } catch {
    return null;
  }
}

export function readActiveWorkspaceId(): string | null {
  const storage = safeStorage();
  if (!storage) {
    return null;
  }
  try {
    const raw = storage.getItem(WORKSPACE_STORAGE_KEY);
    if (!raw) {
      return null;
    }
    const trimmed = raw.trim();
    return trimmed.length > 0 ? trimmed : null;
  } catch {
    return null;
  }
}

export function writeActiveWorkspaceId(workspaceId: string | null): void {
  const storage = safeStorage();
  if (!storage) {
    return;
  }
  try {
    if (!workspaceId) {
      storage.removeItem(WORKSPACE_STORAGE_KEY);
      return;
    }
    storage.setItem(WORKSPACE_STORAGE_KEY, workspaceId);
  } catch {
    // Non-fatal — workspace selection falls back to the in-memory store for this tab.
  }
}
