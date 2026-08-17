import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { useEffect } from "react";

import {
  sessionDetailOptions,
  sessionGoalOptions,
  sessionLedgerOptions,
  sessionRecapOptions,
  sessionUsageOptions,
  sessionsListOptions,
} from "../lib/query-options";
import {
  flattenSessionPages,
  normalizeSessionListFilters,
  sessionListTotal,
} from "../lib/session-list-query";
import type { SessionListFilters, SessionState } from "../types";
import { useActiveWorkspace } from "@/systems/workspace";

interface UseSessionsOptions {
  enabled?: boolean;
  filters?: Omit<SessionListFilters, "workspace">;
  /** Follow the cursor chain until this filtered catalog is complete. */
  loadAll?: boolean;
}

export function useSessions(workspace: string | null = null, options?: UseSessionsOptions) {
  const filters = normalizeSessionListFilters({
    ...options?.filters,
    ...(workspace ? { workspace } : {}),
  });
  const query = useInfiniteQuery({
    ...sessionsListOptions(filters),
    enabled: options?.enabled ?? true,
  });

  useEffect(() => {
    if (
      !options?.loadAll ||
      options.enabled === false ||
      !query.hasNextPage ||
      query.isFetchingNextPage ||
      query.isError
    ) {
      return;
    }
    // React Query records a continuation failure on the query; the consumer
    // reads that state, so the promise itself needs no second error channel.
    void query.fetchNextPage().catch(() => undefined);
  }, [options?.enabled, options?.loadAll, query]);

  return {
    ...query,
    data: flattenSessionPages(query.data),
    total: sessionListTotal(query.data),
  };
}

export function useSession(id: string, workspace?: string | null) {
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const workspaceId = workspace ?? runtimeWorkspaceId ?? "";
  return useQuery(sessionDetailOptions(workspaceId, id));
}

export function useSessionById(id: string, workspace: string) {
  const workspaceId = workspace.trim();
  return useQuery(sessionDetailOptions(workspaceId, id));
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
