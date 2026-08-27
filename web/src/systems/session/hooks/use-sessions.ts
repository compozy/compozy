import { useInfiniteQuery, useQuery } from "@tanstack/react-query";

import {
  sessionScopedDetailOptions,
  sessionGoalOptions,
  sessionLedgerOptions,
  sessionRecapOptions,
  sessionUsageOptions,
  sessionsCompleteListOptions,
  sessionsListOptions,
} from "../lib/query-options";
import {
  flattenSessionPages,
  normalizeSessionListFilters,
  sessionListTotal,
} from "../lib/session-list-query";
import type { SessionListFilters, SessionState } from "../types";
import { useProfileReadScope } from "@/systems/profiles";
import { useActiveWorkspace } from "@/systems/workspace";

interface UseSessionsOptions {
  enabled?: boolean;
  filters?: Omit<
    SessionListFilters,
    "workspace_id" | "all_workspaces" | "profile" | "all_profiles"
  >;
  /** Follow the cursor chain until this filtered catalog is complete. */
  loadAll?: boolean;
}

export function useSessions(workspace: string | null = null, options?: UseSessionsOptions) {
  // The profile scope is applied here rather than at each call site: every
  // session read is server-scoped, and folding it into the filters means the
  // query key partitions by profile for free, so a switch cannot show the
  // previous profile's rows while the new page loads.
  const { params } = useProfileReadScope();
  // Only an explicit `null` means every workspace. An empty string is "no
  // workspace yet" and must not widen into `all_workspaces`.
  const workspaceReady = workspace === null || workspace.trim() !== "";
  const filters = normalizeSessionListFilters({
    ...options?.filters,
    ...(workspace === null ? { all_workspaces: true } : { workspace_id: workspace }),
    ...params,
  });
  const enabled = (options?.enabled ?? true) && workspaceReady;
  const loadAll = options?.loadAll === true;

  const paged = useInfiniteQuery({
    ...sessionsListOptions(filters),
    enabled: enabled && !loadAll,
  });
  const complete = useQuery({
    ...sessionsCompleteListOptions(filters),
    enabled: enabled && loadAll,
  });

  if (loadAll) {
    return {
      ...complete,
      data: complete.data?.sessions,
      total: complete.data?.page.total ?? 0,
      hasNextPage: false,
      isFetchingNextPage: false,
      fetchNextPage: async () => complete,
    };
  }

  return {
    ...paged,
    data: flattenSessionPages(paged.data),
    total: sessionListTotal(paged.data),
  };
}

export function useSession(id: string) {
  const { params } = useProfileReadScope();
  return useQuery(sessionScopedDetailOptions(id, params, { enabled: id.trim() !== "" }));
}

export function useSessionById(id: string, workspace: string) {
  const { params } = useProfileReadScope();
  return useQuery(sessionScopedDetailOptions(id, params, { enabled: workspace.trim() !== "" }));
}

export function useSessionGoal(workspaceId: string, sessionId: string, enabled = true) {
  return useQuery({
    ...sessionGoalOptions(workspaceId, sessionId),
    enabled: enabled && workspaceId !== "" && sessionId !== "",
  });
}

export interface UseSessionLedgerOptions {
  enabled?: boolean;
}

/**
 * The forensic ledger only materializes after `OnSessionEnd`, so the caller
 * must gate this query on `session.state === "stopped"`. Calling it earlier
 * causes a 404 path that lingers as the cached state and prevents the query
 * from naturally fetching when the session later transitions to stopped.
 */
export function useSessionLedger(
  id: string,
  workspace?: string | null,
  options?: UseSessionLedgerOptions
) {
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const workspaceId = workspace ?? runtimeWorkspaceId ?? "";
  return useQuery(sessionLedgerOptions(workspaceId, id, options));
}

export function useSessionRecap(id: string, workspace?: string | null, limit?: number) {
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const workspaceId = workspace ?? runtimeWorkspaceId ?? "";
  return useQuery(sessionRecapOptions(workspaceId, id, limit));
}

export function useSessionUsage(
  id: string,
  workspace?: string | null,
  sessionState?: SessionState | null,
  options?: { enabled?: boolean }
) {
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const workspaceId = workspace ?? runtimeWorkspaceId ?? "";
  return useQuery({
    ...sessionUsageOptions(workspaceId, id, sessionState),
    enabled: Boolean(workspaceId) && Boolean(id) && (options?.enabled ?? true),
  });
}
