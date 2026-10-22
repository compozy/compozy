// Deep import: the workspace barrel reaches the session barrel, and adapters
// sit under every session module, so a barrel edge here would close a cycle.
import { USER_SCOPE_SELECTOR } from "@/systems/workspace/lib/destination";

/**
 * Path segment for a session's workspace-path routes. A session reports User
 * work with an empty `workspace_id`; its routes live under `@user`.
 */
export function sessionWorkspacePath(workspaceId: string): string {
  return workspaceId.trim() === "" ? USER_SCOPE_SELECTOR : workspaceId;
}
