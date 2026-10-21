import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef } from "react";

import { notifyUser } from "@/lib/user-feedback";
import { sessionDetailOptions, SessionNotFoundError } from "@/systems/session";
import {
  focusDestination,
  ownerDataScope,
  useDataScope,
  USER_DESKTOP_WORKSPACE_ID,
  workspaceSelectorForScope,
} from "@/systems/workspace";

import { useDesktop } from "./use-desktop";
import { useOsShell } from "./use-os-shell";
import type { OsOpenTarget } from "../lib/os-types";
import type { RoutingCoordinator } from "../lib/routing-coordinator";
import { windowManagerCommandsAvailable } from "../lib/window-manager-command-availability";

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

interface ResolvedAttentionJumpTarget {
  sessionId: string;
  agentName: string;
  workspaceId: string;
  /** Desktop partition that owns the session: its project, or `user`. */
  partition: string;
  placement?: OsOpenTarget["placement"];
  desktopId?: string;
  created?: boolean;
}

export type AttentionJump = (target: AttentionJumpTarget) => void;

function openAttentionTarget(
  coordinator: RoutingCoordinator,
  target: ResolvedAttentionJumpTarget
): void {
  const open = {
    app: "session" as const,
    instanceKey: target.sessionId,
    route: {
      pathname: `/agents/${encodeURIComponent(target.agentName)}/sessions/${encodeURIComponent(target.sessionId)}`,
      search: {},
    },
    ...(target.placement ? { placement: target.placement } : {}),
    ...(target.desktopId ? { desktopId: target.desktopId } : {}),
  };
  void (
    target.created ? coordinator.userOpenCreatedSession(open) : coordinator.userOpen(open)
  ).catch(() => notifyUser({ message: "Couldn't open this session. Try again.", tone: "error" }));
}

/**
 * One land-on-session behaviour for every attention surface.
 *
 * A foreign activation focuses the session's destination — its project, or
 * User for a session without one — and is held until that partition's window
 * manager is live. Agent notifications may arrive before their session is in
 * an attention list, so the canonical session detail supplies the missing
 * agent route.
 */
export function useAttentionJump(): AttentionJump {
  const queryClient = useQueryClient();
  const { coordinator } = useOsShell();
  const dataScope = useDataScope();
  const commandWorkspaceId = useDesktop(state =>
    windowManagerCommandsAvailable(state) ? state.snapshot?.workspaceId.trim() || null : null
  );
  const pendingRef = useRef<ResolvedAttentionJumpTarget | null>(null);
  const activationRef = useRef(0);
  // The focused partition, or null under All projects and while unsettled.
  const focusedPartition =
    dataScope?.kind === "user"
      ? USER_DESKTOP_WORKSPACE_ID
      : dataScope?.kind === "project"
        ? dataScope.workspaceId
        : null;

  const queueResolvedTarget = (target: ResolvedAttentionJumpTarget) => {
    if (focusedPartition !== target.partition) {
      pendingRef.current = target;
      focusDestination(ownerDataScope(target.workspaceId));
      return;
    }
    if (commandWorkspaceId !== target.partition) {
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
      commandWorkspaceId !== pending.partition ||
      focusedPartition !== pending.partition
    ) {
      return;
    }
    pendingRef.current = null;
    openAttentionTarget(coordinator, pending);
  }, [commandWorkspaceId, coordinator, focusedPartition]);

  return target => {
    const sessionId = target.sessionId.trim();
    const workspaceId = target.workspaceId.trim();
    const scope = ownerDataScope(workspaceId);
    const partition = scope.kind === "user" ? USER_DESKTOP_WORKSPACE_ID : scope.workspaceId;
    const agentName = target.agentName?.trim();
    const placement = target.placement;
    const desktopId = target.desktopId?.trim() || undefined;
    const created = target.created;
    const activation = ++activationRef.current;
    if (sessionId === "") {
      notifyUser({ message: "Couldn't open this session. Try again.", tone: "error" });
      return;
    }
    const resolved = { sessionId, workspaceId, partition, placement, desktopId, created };
    if (agentName) {
      queueResolvedTarget({ ...resolved, agentName });
      return;
    }

    void queryClient
      .ensureQueryData(sessionDetailOptions(workspaceSelectorForScope(scope), sessionId))
      .then(session => {
        if (activation !== activationRef.current) return;
        const resolvedAgentName = session.agent_name.trim();
        if (resolvedAgentName === "") {
          notifyUser({ message: "Couldn't open this session. Try again.", tone: "error" });
          return;
        }
        queueResolvedTarget({ ...resolved, agentName: resolvedAgentName });
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
