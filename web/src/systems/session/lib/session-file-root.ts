import { USER_SCOPE_SELECTOR } from "@/systems/workspace/lib/destination";

/**
 * Whether a session has a file root: its bound worktree or its project checkout. User-scope
 * sessions (`@user`, an empty `workspace_id`) have none, so they get no Review panel and no
 * viewer links (US-001.EC-1, US-005.EC-5).
 */
export function sessionHasFileRoot(workspaceId: string | null | undefined): boolean {
  const id = workspaceId?.trim() ?? "";
  return id !== "" && id !== USER_SCOPE_SELECTOR;
}
