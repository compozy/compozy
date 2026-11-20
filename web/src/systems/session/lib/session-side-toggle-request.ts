/**
 * The `session.side.toggle` shortcut and palette command act on the focused session window only
 * (UT-054). The client op names that window; the window's side chat host listens for its own id.
 */
const listeners = new Map<string, Set<() => void>>();

export function requestSessionSideToggle(windowId: string): void {
  for (const listener of listeners.get(windowId) ?? []) listener();
}

/** Subscribes one window's handler; returns the unsubscribe. */
export function subscribeSessionSideToggle(windowId: string, listener: () => void): () => void {
  const set = listeners.get(windowId) ?? new Set<() => void>();
  set.add(listener);
  listeners.set(windowId, set);
  return () => {
    set.delete(listener);
    if (set.size === 0) listeners.delete(windowId);
  };
}
