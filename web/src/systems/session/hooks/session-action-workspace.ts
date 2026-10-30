import { SessionApiError } from "../adapters/session-api-errors";
import { ownerDataScope, workspaceSelectorForScope } from "@/systems/workspace";

/**
 * The session's own workspace path segment. An empty `workspace_id` is a User
 * session, addressed through `@user`; only an absent value has no target.
 */
export function requireWorkspace(workspaceId: string | null | undefined): string {
  if (workspaceId === null || workspaceId === undefined) {
    throw new SessionApiError("No active workspace selected", 400);
  }
  return workspaceSelectorForScope(ownerDataScope(workspaceId));
}

export interface UseSessionWorkspaceOptions {
  workspaceId?: string | null;
}

/** The caller's explicit workspace, else the active one; `null` when neither is known. */
export function resolveWorkspaceId(
  workspaceId: string | null | undefined,
  runtimeWorkspaceId: string | null | undefined
): string | null {
  return workspaceId ?? runtimeWorkspaceId ?? null;
}
