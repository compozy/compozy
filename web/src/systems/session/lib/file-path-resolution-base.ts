import type { QueryClient } from "@tanstack/react-query";

import { workspaceDetailOptions, worktreesListOptions } from "@/systems/workspace";

import type { FilePathBase } from "./file-path-links";
import { sessionKeys } from "./query-keys";
import { sessionDetailOptions } from "./query-options";
import type { SessionPayload } from "../types";

export interface SessionScope {
  sessionId: string;
  workspaceId: string;
}

// An invalidated entry (e.g. after a worktree change) may name a stale `working_dir`, so only
// valid entries count; with none, the click rereads the detail.
function cachedSessionDetail(
  queryClient: QueryClient,
  { sessionId, workspaceId }: SessionScope
): SessionPayload | undefined {
  const cache = queryClient.getQueryCache();
  const detail = cache.find({ queryKey: sessionKeys.detail(workspaceId, sessionId), exact: true });
  const queries = cache.findAll({ queryKey: sessionKeys.byIdRoot(sessionId) });
  if (detail) queries.push(detail);
  const candidates: SessionPayload[] = [];
  for (const query of queries) {
    const data = query.state.data as SessionPayload | undefined;
    if (!query.state.isInvalidated && data !== undefined) candidates.push(data);
  }
  return candidates.find(data => data.working_dir) ?? candidates[0];
}

// Detail-cache writers (rename, live-tail) may replace the payload without `working_dir`, so the
// click reads it fresh whenever the cached detail lacks it (spec Data Models, web cache rule).
async function readSessionDetail(
  queryClient: QueryClient,
  scope: SessionScope
): Promise<SessionPayload | undefined> {
  const cached = cachedSessionDetail(queryClient, scope);
  if (cached?.working_dir) return cached;
  try {
    return await queryClient.fetchQuery({
      ...sessionDetailOptions(scope.workspaceId, scope.sessionId),
      staleTime: 0,
    });
  } catch {
    return cached;
  }
}

async function readOptional<T>(read: () => Promise<T>): Promise<T | undefined> {
  try {
    return await read();
  } catch {
    return undefined;
  }
}

/**
 * The bases a transcript path resolves against, read at click time: the session's working
 * directory, its bound worktree's path, and the project root; `roots` lists the inside roots.
 * `worktreeId` is the session's bound worktree, taken from the session itself so an unreadable
 * listing never turns it into the checkout; `null` only for a session bound to none.
 */
export async function readFilePathResolutionBase(queryClient: QueryClient, scope: SessionScope) {
  const session = await readSessionDetail(queryClient, scope);
  const worktreeId = session?.worktree_id?.trim() ?? "";
  const [workspace, worktrees] = await Promise.all([
    readOptional(() => queryClient.ensureQueryData(workspaceDetailOptions(scope.workspaceId))),
    worktreeId === ""
      ? Promise.resolve(undefined)
      : readOptional(() => queryClient.ensureQueryData(worktreesListOptions(scope.workspaceId))),
  ]);
  const base: FilePathBase = {
    workingDir: session?.working_dir,
    worktreePath: worktrees?.worktrees.find(entry => entry.id === worktreeId)?.path,
    workspaceRoot: workspace?.workspace.root_dir,
  };
  const roots = [base.worktreePath, base.workspaceRoot]
    .map(root => root?.trim() ?? "")
    .filter((root, index, all) => root !== "" && all.indexOf(root) === index);
  return { base, roots, worktreeId: worktreeId === "" ? null : worktreeId };
}
