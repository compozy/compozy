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
 */
export function useSessionWindowRuntime(windowId: string): SessionWindowRuntime {
  const { liveTailEnabled, pathname } = useSessionWindowDesktopState(windowId);
  const focused = useDesktop(state => state.focusedId === windowId);
  const documentActive = useDocumentActive();
  const sessionId = matchSessionInstance(pathname);
  const agentMatch = SESSION_AGENT_PATTERN.exec(pathname);
  return {
    liveTailEnabled,
    presenceEnabled: focused && liveTailEnabled && documentActive,
    sessionId,
    agentName: agentMatch ? decodeURIComponent(agentMatch[1]) : null,
  };
}
