import { matchSessionInstance } from "../lib/app-catalog";
import type { OsDesktopRuntimeStore } from "../lib/os-types";
import { useDesktop } from "./use-desktop";

/**
 * The session the operator is looking at right now, or null.
 *
 * "Looking at" means the focused window is showing a session *and* that window
 * is on the active desktop, un-minimized, and the front tab of its stack. This
 * is the client half of focus suppression: the daemon knows a session is
 * watched through the presence lease, and this is what stops the shell
 * announcing something already on screen.
 */
export function selectFocusedSessionId(state: OsDesktopRuntimeStore): string | null {
  const windowId = state.focusedId;
  if (windowId === null) return null;
  const focused = state.windows[windowId];
  if (
    focused === undefined ||
    focused.desktopId !== state.activeDesktopId ||
    focused.minimized ||
    !focused.stackActive
  ) {
    return null;
  }
  return matchSessionInstance(focused.route.pathname);
}

export function useFocusedSessionId(): string | null {
  return useDesktop(selectFocusedSessionId);
}
