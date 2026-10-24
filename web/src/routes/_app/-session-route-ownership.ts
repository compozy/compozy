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
  /**
   * The All projects desktop opens every session in place, bound to its own
   * scope (route search `workspace`); other desktops need no binding.
   */
  bindsInPlace: boolean;
}

/**
 * The User desktop proves ownership through `@user` (the request's profile) and
 * owns sessions with no project. A project desktop owns its project. The All
 * projects desktop (`global`) owns no data scope: every session opens there in
 * place, bound to its own scope, resolved by the owner projection.
 */
export function sessionRouteOwnership(
  selection: Pick<ActiveWorkspaceResolution, "desktopWorkspaceId">
): SessionRouteOwnership | null {
  const partition = selection.desktopWorkspaceId;
  if (!partition) return null;
  if (partition === USER_DESKTOP_WORKSPACE_ID) {
    return {
      ownerWorkspaceId: "",
      proofSelector: USER_SCOPE_SELECTOR,
      accepts: workspaceId => workspaceId === "",
      bindsInPlace: false,
    };
  }
  if (partition === GLOBAL_DESKTOP_WORKSPACE_ID) {
    return {
      ownerWorkspaceId: partition,
      proofSelector: null,
      accepts: () => true,
      bindsInPlace: true,
    };
  }
  return {
    ownerWorkspaceId: partition,
    proofSelector: partition,
    accepts: workspaceId => workspaceId === partition,
    bindsInPlace: false,
  };
}
