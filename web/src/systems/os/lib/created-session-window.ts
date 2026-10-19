import type { OsOpenTarget, OsWindow, OsWindowRoute, WindowManagerOpenOutcome } from "./os-types";
import type { RoutingManager } from "./routing-coordinator-contract";
import { mruWindowInstance } from "./window-instance-lookup";

/** A just-created session: its window key and the route it opens on. */
export interface CreatedSessionTarget {
  instanceKey: string;
  route: OsWindowRoute;
  /** The desktop the session lands on; omitted uses the client's active desktop. */
  desktopId?: string;
}

type HostState = Pick<ReturnType<RoutingManager["getState"]>, "client" | "windows" | "focusedId">;

/** A retired or never-keyed session window shows the empty state (`/sessions`). */
function showsEmptyState(win: OsWindow): boolean {
  return (win.instanceKey ?? "") === "";
}

/**
 * New sessions land in a Session window the operator already has (#825) instead of spawning
 * another one. The host is, in order:
 *
 * 1. the focused Session window, which keeps focus;
 * 2. the most recently focused Session window showing the empty state, so no conversation
 *    leaves the screen;
 * 3. the most recently focused Session window, restored when minimized.
 *
 * Only windows on the landing desktop qualify: reaching across desktops would switch the
 * client away from where the operator started. Pinned windows keep what they were pinned to.
 * `null` keeps the plain open — no candidate, the session already owns a window (focus wins),
 * or the caller named an explicit destination (split, tab, new instance).
 */
export function createdSessionHostWindow(
  state: HostState,
  target: CreatedSessionTarget &
    Pick<OsOpenTarget, "forceNewInstance" | "stackTargetWindowId" | "placement">
): OsWindow | null {
  if (target.forceNewInstance || target.stackTargetWindowId || target.placement) return null;
  const client = state.client;
  if (client === null || target.instanceKey === "") return null;
  const owner = mruWindowInstance(state.windows, client.focusOrder, {
    app: "session",
    instanceKey: target.instanceKey,
  });
  if (owner) return null;
  const desktopId = target.desktopId ?? client.activeDesktopId;
  const candidates: Record<string, OsWindow> = {};
  const empty: Record<string, OsWindow> = {};
  for (const win of Object.values(state.windows)) {
    if (win.app !== "session" || win.desktopId !== desktopId || win.pinned) continue;
    candidates[win.id] = win;
    if (showsEmptyState(win)) empty[win.id] = win;
  }
  const focused = state.focusedId !== null ? candidates[state.focusedId] : undefined;
  if (focused && !focused.minimized) return focused;
  return (
    mruWindowInstance(empty, client.focusOrder, { app: "session" }) ??
    mruWindowInstance(candidates, client.focusOrder, { app: "session" })
  );
}

/**
 * Re-keys `host` to the created session in place — its tab, split and frame stay where they
 * are — then activates it: restore when minimized, focus otherwise, nothing when it already
 * is the active focused window. The activation waits on the retarget so the window never
 * gains focus while it still shows the previous session.
 */
export function landCreatedSession(
  manager: Pick<RoutingManager, "getState" | "retargetWindow" | "focusWindow" | "restoreWindow">,
  host: OsWindow,
  target: CreatedSessionTarget
): WindowManagerOpenOutcome {
  const retarget = manager.retargetWindow(host.id, target.instanceKey, target.route);
  if (!retarget.accepted) return { windowId: host.id, ...retarget };
  const completion = retarget.completion.then(async applied => {
    if (!applied) return false;
    const state = manager.getState();
    const win = state.windows[host.id];
    if (!win) return false;
    if (state.focusedId === host.id && !win.minimized && win.stackActive) return true;
    const activation = win.minimized
      ? manager.restoreWindow(host.id)
      : manager.focusWindow(host.id);
    return activation.accepted && (await activation.completion);
  });
  return { windowId: host.id, accepted: true, completion };
}
