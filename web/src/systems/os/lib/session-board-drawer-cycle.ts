export type SessionBoardDrawerCycleDirection = "previous" | "next";

const cycleHandlers = new Map<string, (direction: SessionBoardDrawerCycleDirection) => void>();

/**
 * A session window whose board shows a session in its drawer (#1052) claims the shell's
 * Previous/Next session shortcut while that window is focused: the shortcut then steps
 * through the board's cards instead of the session list. Returns the release.
 */
export function claimSessionBoardDrawerCycle(
  windowId: string,
  handler: (direction: SessionBoardDrawerCycleDirection) => void
): () => void {
  cycleHandlers.set(windowId, handler);
  return () => {
    if (cycleHandlers.get(windowId) === handler) cycleHandlers.delete(windowId);
  };
}

/** Steps the focused window's board drawer; `false` when that window holds no open drawer. */
export function cycleSessionBoardDrawer(
  windowId: string | null,
  direction: SessionBoardDrawerCycleDirection
): boolean {
  const handler = windowId === null ? undefined : cycleHandlers.get(windowId);
  if (!handler) return false;
  handler(direction);
  return true;
}
