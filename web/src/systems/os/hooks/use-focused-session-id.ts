import { use, useSyncExternalStore } from "react";

import { OsShellContext } from "../contexts/os-shell-context";
import { matchSessionInstance } from "../lib/app-catalog";
import type { OsDesktopRuntimeStore } from "../lib/os-types";
import { useDesktop } from "./use-desktop";

/**
 * The session the operator is looking at right now, or null.
 *
 * "Looking at" means the focused window is showing a session *and* that window
 * is on the active desktop and on screen: its tab is active and no collapsed split hides it. This
 * is the client half of focus suppression: the daemon knows a session is
 * watched through the presence lease, and this is what stops the shell
 * announcing something already on screen.
 */
export function selectFocusedSessionId(state: OsDesktopRuntimeStore): string | null {
  const windowId = state.focusedId;
  if (windowId === null) return null;
  const focused = state.windows[windowId];
  if (focused === undefined || focused.desktopId !== state.activeDesktopId || !focused.shown) {
    return null;
  }
  return matchSessionInstance(focused.route.pathname);
}

export function useFocusedSessionId(): string | null {
  return useDesktop(selectFocusedSessionId);
}

const noSubscription = () => () => undefined;
const noSession = () => null;

/**
 * `useFocusedSessionId` for surfaces that may also render outside the OS shell (a standalone
 * route, a test): there it is always null.
 */
export function useShellFocusedSessionId(): string | null {
  const projection = use(OsShellContext)?.projection;
  return useSyncExternalStore(
    projection
      ? onChange => {
          const subscription = projection.subscribe(onChange);
          return () => subscription.unsubscribe();
        }
      : noSubscription,
    projection ? () => selectFocusedSessionId(projection.get()) : noSession,
    noSession
  );
}
