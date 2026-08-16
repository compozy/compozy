import { useDesktop } from "../../../hooks/use-desktop";
import { matchSessionInstance } from "../../../lib/app-catalog";
import { useSessionWindowDesktopState } from "./use-session-window-desktop-state";
import { useSessionPresence } from "@/systems/session";
import { useActiveWorkspace } from "@/systems/workspace";

const SESSION_AGENT_PATTERN = /^\/agents\/([^/]+)\/sessions\//;

export interface SessionWindowRuntime {
  runtimeWorkspaceId: string | null;
  liveTailEnabled: boolean;
  sessionId: string | null;
  agentName: string | null;
}

/**
 * Everything the session window needs from the shell: which session its route
 * points at, whether its data should stay live, and — while the operator is
 * actually looking at it — an operator presence lease.
 *
 * Presence is reported only when this window holds focus *and* its content is
 * visible. That pairing is the whole definition of "being viewed": a focused
 * window on a hidden tab or a background desktop is not someone watching, and
 * claiming otherwise would silently stop `done` from ever appearing.
 */
export function useSessionWindowRuntime(windowId: string): SessionWindowRuntime {
  const activeWorkspace = useActiveWorkspace();
  const { liveTailEnabled, pathname } = useSessionWindowDesktopState(windowId);
  const focused = useDesktop(state => state.focusedId === windowId);
  const runtimeWorkspaceId = activeWorkspace.runtimeWorkspaceId?.trim() || null;
  const sessionId = matchSessionInstance(pathname);
  const agentMatch = SESSION_AGENT_PATTERN.exec(pathname);
  useSessionPresence(runtimeWorkspaceId, sessionId, focused && liveTailEnabled);
  return {
    runtimeWorkspaceId,
    liveTailEnabled,
    sessionId,
    agentName: agentMatch ? decodeURIComponent(agentMatch[1]) : null,
  };
}
