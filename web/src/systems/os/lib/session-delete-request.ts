import type { SessionPayload } from "@/systems/session";

/** One delete the window rail asks a stable host (the OS shell) to confirm. */
export interface SessionDeleteRequest {
  sessions: readonly SessionPayload[];
  bulk: boolean;
  onSelectionChange?: (remainingIds: readonly string[]) => void;
}

type SessionDeleteRequestListener = (request: SessionDeleteRequest) => void;

const listeners = new Set<SessionDeleteRequestListener>();

/**
 * Offers a delete confirmation to the stable shell host. The session window's own lifecycle
 * host cannot own it: the daemon reconciles the deleted session's window before the DELETE
 * answers, so a window-hosted dialog would leave with the window mid-request. Returns false
 * when no host listens (tests, stories), so the caller keeps its own dialog as a fallback.
 */
export function requestSessionDelete(request: SessionDeleteRequest): boolean {
  if (listeners.size === 0) return false;
  for (const listener of listeners) listener(request);
  return true;
}

/** Hears delete requests while mounted; the shell's lifecycle owner registers exactly one. */
export function subscribeSessionDeleteRequests(listener: SessionDeleteRequestListener): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}
