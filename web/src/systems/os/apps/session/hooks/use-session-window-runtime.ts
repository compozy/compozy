import { useDeferredValue } from "react";

import { useDesktop } from "../../../hooks/use-desktop";
import { matchSessionInstance } from "../../../lib/app-catalog";
import { useSessionWindowDesktopState } from "./use-session-window-desktop-state";
import { useDocumentActive } from "@/hooks/use-document-active";

const SESSION_AGENT_PATTERN = /^\/agents\/([^/]+)\/sessions\//;

export interface SessionWindowRuntime {
  liveTailEnabled: boolean;
  presenceEnabled: boolean;
  sessionId: string | null;
  agentName: string | null;
  /** The route asks the window's board to show the session in its drawer (#1052). */
  drawerOpen: boolean;
}

/**
 * Everything the session window needs from the shell: which session its route
 * points at, whether its data should stay live, and whether the operator is
 * actually looking at it. The controller combines that eligibility with the
 * workspace resolved from the authoritative session read before leasing presence.
 *
 * Presence is reported only when this window holds focus *and* its content is
 * visible. That pairing is the whole definition of "being viewed": a focused
 * window on a hidden tab or a background desktop is not someone watching, and
 * claiming otherwise would silently stop `done` from ever appearing.
 *
 * The session identity follows the route one deferred render behind it. The route comes from
 * the shell's store, whose updates are always urgent, and an urgent update can never animate
 * (`<ViewTransition>` only plays for transitions). Deferring the path turns the switch to
 * another session into one: the window keeps showing the current session until the next is
 * ready, then swaps under the keyed boundary in `SessionWindowView`.
 *
 * The session is being viewed only while it is on screen: always in List, and on the
 * Kanban view only while the board shows it in its drawer.
 */
export function useSessionWindowRuntime(windowId: string): SessionWindowRuntime {
  const live = useSessionWindowDesktopState(windowId);
  const { liveTailEnabled } = live;
  // The drawer flag defers with the path: opening a card must not show the drawer a frame
  // early with the previous session in it.
  const location = useDeferredValue(`${live.drawerOpen ? "1" : "0"}${live.pathname}`);
  const drawerOpen = location.startsWith("1");
  const pathname = location.slice(1);
  const focused = useDesktop(state => state.focusedId === windowId);
  const documentActive = useDocumentActive();
  const sessionId = matchSessionInstance(pathname);
  const agentMatch = SESSION_AGENT_PATTERN.exec(pathname);
  return {
    liveTailEnabled,
    presenceEnabled: focused && liveTailEnabled && documentActive,
    sessionId,
    agentName: agentMatch ? decodeURIComponent(agentMatch[1]) : null,
    drawerOpen,
  };
}
