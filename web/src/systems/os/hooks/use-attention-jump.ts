import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef } from "react";

import { notifyUser } from "@/lib/user-feedback";
import { sessionDetailOptions, SessionNotFoundError } from "@/systems/session";
import { useActiveWorkspace } from "@/systems/workspace";

import { useDesktop } from "./use-desktop";
import { useOsShell } from "./use-os-shell";
import type { RoutingCoordinator } from "../lib/routing-coordinator";
import { windowManagerCommandsAvailable } from "../lib/window-manager-command-availability";

export interface AttentionJumpTarget {
  sessionId: string;
  agentName?: string;
  workspaceId: string;
}

interface ResolvedAttentionJumpTarget {
  sessionId: string;
  agentName: string;
  workspaceId: string;
}

export type AttentionJump = (target: AttentionJumpTarget) => void;

function openAttentionTarget(
  coordinator: RoutingCoordinator,
  target: ResolvedAttentionJumpTarget
): void {
  void coordinator
    .userOpen({
      app: "session",
      instanceKey: target.sessionId,
      route: {
        pathname: `/agents/${encodeURIComponent(target.agentName)}/sessions/${encodeURIComponent(target.sessionId)}`,
        search: {},
      },
    })
    .catch(() => notifyUser({ message: "Couldn't open this session. Try again.", tone: "error" }));
}

/**
 * One land-on-session behaviour for every attention surface.
 *
 * A foreign activation is held until the target workspace's window manager is
 * live. Agent notifications may arrive before their session is in an attention
 * list, so the canonical session detail supplies the missing agent route.
 */
export function useAttentionJump(): AttentionJump {
  const queryClient = useQueryClient();
  const { coordinator } = useOsShell();
  const { runtimeWorkspaceId, setActiveWorkspaceId } = useActiveWorkspace();
  const commandWorkspaceId = useDesktop(state =>
    windowManagerCommandsAvailable(state) ? state.snapshot?.workspaceId.trim() || null : null
  );
  const pendingRef = useRef<ResolvedAttentionJumpTarget | null>(null);
  const activationRef = useRef(0);

  const queueResolvedTarget = (target: ResolvedAttentionJumpTarget) => {
    if (target.workspaceId !== runtimeWorkspaceId?.trim()) {
      pendingRef.current = target;
      setActiveWorkspaceId(target.workspaceId);
      return;
    }
    if (commandWorkspaceId !== target.workspaceId) {
      pendingRef.current = target;
      return;
    }
    pendingRef.current = null;
    openAttentionTarget(coordinator, target);
  };

  useEffect(() => {
    const pending = pendingRef.current;
    if (
      pending === null ||
      commandWorkspaceId !== pending.workspaceId ||
      pending.workspaceId !== runtimeWorkspaceId?.trim()
    ) {
      return;
    }
    pendingRef.current = null;
    openAttentionTarget(coordinator, pending);
  }, [commandWorkspaceId, coordinator, runtimeWorkspaceId]);

  return target => {
    const sessionId = target.sessionId.trim();
    const workspaceId = target.workspaceId.trim();
    const agentName = target.agentName?.trim();
    const activation = ++activationRef.current;
    if (sessionId === "" || workspaceId === "") {
      notifyUser({ message: "Couldn't open this session. Try again.", tone: "error" });
      return;
    }
    if (agentName) {
      queueResolvedTarget({ sessionId, workspaceId, agentName });
      return;
    }

    void queryClient
      .ensureQueryData(sessionDetailOptions(workspaceId, sessionId))
      .then(session => {
        if (activation !== activationRef.current) return;
        const resolvedAgentName = session.agent_name.trim();
        if (resolvedAgentName === "") {
          notifyUser({ message: "Couldn't open this session. Try again.", tone: "error" });
          return;
        }
        queueResolvedTarget({ sessionId, workspaceId, agentName: resolvedAgentName });
      })
      .catch(error => {
        if (activation !== activationRef.current) return;
        notifyUser({
          message:
            error instanceof SessionNotFoundError
              ? "Session not found"
              : "Couldn't open this session. Try again.",
          tone: "error",
        });
      });
  };
}
