import { subagentRevealStore } from "@/systems/session";
import {
  ownerDataScope,
  sameDataScope,
  useDataScope,
  useWindowDataWorkspace,
} from "@/systems/workspace";

import { useAttentionJump } from "../../hooks/use-attention-jump";
import { useOsShell } from "../../hooks/use-os-shell";
import { boundRoute } from "../../lib/open-target";

/** A session to open by id; the agent name lets this window retarget in place. */
export interface SessionWindowSidebarTarget {
  sessionId: string;
  agentName: string;
  workspaceId: string;
  subagentId?: string;
}

/**
 * Opens a session from inside a session window (rail row, board card). It
 * retargets this window in place — rebinding it under All projects — unless the
 * target belongs elsewhere or already owns a window: that window wins focus.
 */
export function useSessionWindowOpenSession({
  windowId,
  sessionId,
}: {
  windowId: string;
  sessionId?: string;
}) {
  const scope = useDataScope();
  const windowScope = useWindowDataWorkspace();
  const { coordinator } = useOsShell();
  const jumpToSession = useAttentionJump();

  return (target: SessionWindowSidebarTarget, options: { newWindow: boolean }) => {
    if (target.subagentId) {
      subagentRevealStore.trigger.requested({
        workspaceId: target.workspaceId,
        sessionId: target.sessionId,
        subagentId: target.subagentId,
      });
    }
    if (sessionId !== undefined && target.sessionId === sessionId) return;
    const targetScope = ownerDataScope(target.workspaceId);
    const allProjects = scope?.kind === "all";
    const sameScope = windowScope !== null && sameDataScope(windowScope, targetScope);
    if (options.newWindow || !target.agentName || (!allProjects && !sameScope)) {
      jumpToSession({
        sessionId: target.sessionId,
        agentName: target.agentName || undefined,
        workspaceId: target.workspaceId,
        ...(options.newWindow ? { placement: "split" as const } : {}),
      });
      return;
    }
    const route = {
      pathname: `/agents/${encodeURIComponent(target.agentName)}/sessions/${encodeURIComponent(target.sessionId)}`,
      search: {},
    };
    // Same window, different conversation. The shell's store applies the retarget, and
    // `useSessionWindowRuntime` defers the session identity it feeds, so the switch is the
    // one transition `SessionWindowView`'s boundary animates; there is nothing to wrap here.
    void coordinator.userRetarget(windowId, {
      app: "session",
      instanceKey: target.sessionId,
      // Under All projects the window follows the session it now shows.
      route: allProjects ? boundRoute({ binding: targetScope, route }) : route,
    });
  };
}
