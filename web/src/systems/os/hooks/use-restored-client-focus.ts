import { useEffect, useEffectEvent, useRef } from "react";

import type { WindowManagerRuntime } from "../runtime/window-manager-runtime";

export interface RestoredClientFocusInput {
  manager: Pick<WindowManagerRuntime, "getState" | "focusWindow">;
  /** The window a lost registration had focused, until it settles (`useWindowManagerClient`). */
  restoredFocus: string | null;
  /** Ends the restore of `windowId`. */
  settle: (windowId: string) => void;
  /** The adopted snapshot's revision: a refused focus is tried again on the next one. */
  snapshotRevision: number | null;
  /** The registered client; a new one re-runs the restore. */
  client: unknown;
}

/**
 * Refocuses the window this page had focused after a re-registration that follows a lost one (a
 * daemon restart, #1655), once the restored view is adopted. The answer is authoritative state:
 * its report replaces the URL to that window and adds no history entry.
 *
 * The restore is kept until the focus applies or the window is gone: a focus the daemon refuses
 * (sent against a snapshot older than the restarted daemon's) is tried again on the next snapshot
 * revision. It gives up once focus moves anywhere else meanwhile: the operator (or another
 * client) chose a window, and a late restore must not take it back.
 */
export function useRestoredClientFocus({
  manager,
  restoredFocus,
  settle,
  snapshotRevision,
  client,
}: RestoredClientFocusInput): void {
  const settleRestore = useEffectEvent(settle);
  const inFlight = useRef(false);
  /** The restore under way and the focus the restored view had when it began. */
  const armed = useRef<{ target: string; focusedId: string | null } | null>(null);
  useEffect(() => {
    if (restoredFocus === null) {
      armed.current = null;
      return;
    }
    if (snapshotRevision === null || inFlight.current || client === null) return;
    const state = manager.getState();
    if (state.client === null) return;
    if (armed.current?.target !== restoredFocus) {
      armed.current = { target: restoredFocus, focusedId: state.focusedId };
    }
    const movedElsewhere =
      state.focusedId !== armed.current.focusedId && state.focusedId !== restoredFocus;
    if (!state.windows[restoredFocus] || state.focusedId === restoredFocus || movedElsewhere) {
      armed.current = null;
      settleRestore(restoredFocus);
      return;
    }
    inFlight.current = true;
    void manager
      .focusWindow(restoredFocus, { replaceable: true })
      .completion.catch(() => false)
      .then(applied => {
        inFlight.current = false;
        if (applied) settleRestore(restoredFocus);
      });
  }, [client, manager, restoredFocus, snapshotRevision]);
}
