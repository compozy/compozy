import { useQueryClient } from "@tanstack/react-query";
import { useRef } from "react";

import { notifyUser } from "@/lib/user-feedback";
import { sessionDetailOptions, SessionNotFoundError } from "@/systems/session";
import { ownerDataScope, workspaceSelectorForScope } from "@/systems/workspace";

import type { OsOpenTarget } from "../lib/os-types";
import { useOpenTarget } from "./use-open-target";

export interface AttentionJumpTarget {
  sessionId: string;
  agentName?: string;
  /** The session's own `workspace_id`; empty for a User session. */
  workspaceId: string;
  /**
   * Where the session opens when it has no window yet. Omitted follows the
   * new-window policy (a tab in the focused window); a surface that promises
   * to keep the current window visible asks for `split`.
   */
  placement?: OsOpenTarget["placement"];
  /**
   * The desktop the session opens on when it has no window yet. Omitted uses
   * the active desktop; a launcher that started on a specific desktop names it
   * so a desktop switch while the session is created cannot move it.
   */
  desktopId?: string;
  /**
   * The session was just created by this launcher: it lands in an existing Session window
   * when one qualifies instead of opening another (#825).
   */
  created?: boolean;
}

export type AttentionJump = (target: AttentionJumpTarget) => void;

const OPEN_FAILURE = "Couldn't open this session. Try again.";

function sessionRoute(agentName: string, sessionId: string) {
  return {
    pathname: `/agents/${encodeURIComponent(agentName)}/sessions/${encodeURIComponent(sessionId)}`,
    search: {},
  };
}

/**
 * One land-on-session behaviour for every attention surface, through
 * `openTarget`: under All projects the session opens in place in the `global`
 * desktop, bound to its scope; with a project or User focused, a foreign
 * session focuses its destination first. Agent notifications may arrive
 * before their session is in an attention list, so the canonical session
 * detail supplies the missing agent route.
 */
export function useAttentionJump(): AttentionJump {
  const queryClient = useQueryClient();
  const open = useOpenTarget();
  const activationRef = useRef(0);

  return target => {
    const sessionId = target.sessionId.trim();
    const binding = ownerDataScope(target.workspaceId);
    const activation = ++activationRef.current;
    if (sessionId === "") {
      notifyUser({ message: OPEN_FAILURE, tone: "error" });
      return;
    }
    const land = (agentName: string) =>
      open(
        {
          app: "session",
          instanceKey: sessionId,
          binding,
          route: sessionRoute(agentName, sessionId),
          ...(target.placement ? { placement: target.placement } : {}),
          ...(target.desktopId?.trim() ? { desktopId: target.desktopId.trim() } : {}),
          ...(target.created ? { created: true } : {}),
        },
        OPEN_FAILURE
      );
    const agentName = target.agentName?.trim();
    if (agentName) {
      land(agentName);
      return;
    }

    void queryClient
      .ensureQueryData(sessionDetailOptions(workspaceSelectorForScope(binding), sessionId))
      .then(session => {
        if (activation !== activationRef.current) return;
        const resolvedAgentName = session.agent_name.trim();
        if (resolvedAgentName === "") {
          notifyUser({ message: OPEN_FAILURE, tone: "error" });
          return;
        }
        land(resolvedAgentName);
      })
      .catch(error => {
        if (activation !== activationRef.current) return;
        notifyUser({
          message: error instanceof SessionNotFoundError ? "Session not found" : OPEN_FAILURE,
          tone: "error",
        });
      });
  };
}
