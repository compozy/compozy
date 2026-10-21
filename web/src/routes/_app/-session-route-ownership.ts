import {
  GLOBAL_DESKTOP_WORKSPACE_ID,
  USER_DESKTOP_WORKSPACE_ID,
  USER_SCOPE_SELECTOR,
  type ActiveWorkspaceResolution,
} from "@/systems/workspace";

/**
 * Which sessions a mounted desktop may open without asking. Session windows
 * belong to the desktop partition, not to the catalog lens of the data scope.
 */
export interface SessionRouteOwnership {
  /** Workspace id the desktop's own sessions report; `""` for the User desktop. */
  ownerWorkspaceId: string;
  /**
   * Workspace selector that proves ownership through the workspace-path detail
   * route; `null` when the partition names no data scope (`global`) and only
   * the owner projection can answer.
   */
  proofSelector: string | null;
  /** True when a session reporting `workspaceId` opens here without a switch. */
  accepts: (workspaceId: string) => boolean;
}

/**
 * The User desktop proves ownership through `@user` (the request's profile) and
 * owns sessions with no project. A project desktop owns its project; under All
 * projects it also keeps today's acceptance of User sessions.
 */
export function sessionRouteOwnership(
  selection: Pick<ActiveWorkspaceResolution, "breadthMode" | "desktopWorkspaceId">
): SessionRouteOwnership | null {
  const partition = selection.desktopWorkspaceId;
  if (!partition) return null;
  if (partition === USER_DESKTOP_WORKSPACE_ID) {
    return {
      ownerWorkspaceId: "",
      proofSelector: USER_SCOPE_SELECTOR,
      accepts: workspaceId => workspaceId === "",
    };
  }
  const acceptsUser = selection.breadthMode === "all";
  return {
    ownerWorkspaceId: partition,
    proofSelector: partition === GLOBAL_DESKTOP_WORKSPACE_ID ? null : partition,
    accepts: workspaceId => workspaceId === partition || (acceptsUser && workspaceId === ""),
  };
}
