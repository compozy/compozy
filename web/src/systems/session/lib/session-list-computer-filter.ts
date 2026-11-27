import type { WorkspacePayload } from "@/systems/workspace";
import {
  unknownProjectIdentity,
  workspaceBadgeIdentity,
  type WorkspaceBadgeIdentity,
} from "@/systems/workspace/lib/workspace-badge-identity";
import { ownerDataScope } from "@/systems/workspace/lib/destination";

import { sessionListShelfFilters, type SessionListShelf } from "./session-list-shelf";
import { sessionListSubagentsParam } from "./session-list-preferences";
import type { SessionListFilters, SessionPayload } from "../types";

/** Population filters shared across every computer's facets and list pages (`F5`, `R5`, `R6`). */
export function buildComputerPopulationFilters(options: {
  shelf: SessionListShelf;
  showAutomation: boolean;
  search?: string;
  worktreeId?: string;
  excludePinned?: boolean;
}): SessionListFilters {
  const search = options.search?.trim() ?? "";
  return {
    subagents: sessionListSubagentsParam(search),
    ...(search ? { q: search, search_fields: "title_agent" as const } : {}),
    ...sessionListShelfFilters(options.shelf, options.showAutomation),
    ...(options.excludePinned && options.shelf === "active" ? { pinned: "exclude" as const } : {}),
    ...(options.worktreeId ? { worktree: options.worktreeId } : {}),
  };
}

/** Matches a focused local project onto a remote computer's workspaces via `repository_key` (`Q5`). */
export function matchRemoteWorkspaceIds(
  focusedRepositoryKey: string | null | undefined,
  remoteWorkspaces: readonly WorkspacePayload[]
): readonly string[] {
  const key = focusedRepositoryKey?.trim();
  if (!key) return [];
  const matches: string[] = [];
  for (const workspace of remoteWorkspaces) {
    if (workspace.repository_key?.trim() === key) matches.push(workspace.id);
  }
  return matches;
}

/** In-memory search filter used for cached rows of an offline computer (`R11`). */
export function matchesOfflineSessionQuery(session: SessionPayload, query: string): boolean {
  const normalized = query.trim().toLowerCase();
  if (normalized === "") return true;
  const haystacks = [session.name, session.branch, session.agent_name];
  return haystacks.some(value => value?.toLowerCase().includes(normalized));
}

/**
 * Resolves a project badge for one computer's workspace catalog (`P1`):
 * - User scope stays "User".
 * - Known workspace id shows its project name.
 * - Unresolved remote id falls back to the session's path tail, else "Unknown project".
 * Never renders a raw `ws_…` id for remote rows.
 */
export function computerWorkspaceBadgeIdentity(
  workspaceId: string | null | undefined,
  names: ReadonlyMap<string, string>,
  options: { isRemote?: boolean; workspacePath?: string | null } = {}
): WorkspaceBadgeIdentity {
  const scope = ownerDataScope(workspaceId);
  if (scope.kind === "user") {
    return workspaceBadgeIdentity(workspaceId, names);
  }
  const mapped = names.get(scope.workspaceId)?.trim();
  if (mapped) return { kind: "project", name: mapped };
  if (!options.isRemote) {
    return workspaceBadgeIdentity(workspaceId, names);
  }
  const tail = options.workspacePath?.split("/").filter(Boolean).at(-1)?.trim();
  if (tail) return { kind: "project", name: tail };
  return unknownProjectIdentity();
}
