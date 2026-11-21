import { useInfiniteQuery, useQuery } from "@tanstack/react-query";

import {
  sessionScopedDetailOptions,
  sessionGoalOptions,
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
import { useProfileReadScope } from "@/systems/profiles";
import { useActiveWorkspace } from "@/systems/workspace";
import { useSessionDeletionPending, useSessionDeletedByOperator } from "./use-session-actions";

interface UseSessionsOptions {
  enabled?: boolean;
  filters?: Omit<
    SessionListFilters,
    "workspace_id" | "all_workspaces" | "profile" | "all_profiles"
  >;
}

export function useSessions(workspace: string | null = null, options?: UseSessionsOptions) {
  // The profile scope is applied here rather than at each call site: every
  // session read is server-scoped, and folding it into the filters means the
  // query key partitions by profile for free, so a switch cannot show the
  // previous profile's rows while the new page loads.
  const { params } = useProfileReadScope();
  const filters = normalizeSessionListFilters({
    ...options?.filters,
    ...(workspace ? { workspace_id: workspace } : { all_workspaces: true }),
    ...params,
  });
  const query = useInfiniteQuery({
    ...sessionsListOptions(filters),
    enabled: options?.enabled ?? true,
  });

  return {
    ...query,
    data: flattenSessionPages(query.data),
    total: sessionListTotal(query.data),
  };
}

export function useSession(id: string) {
  const { params } = useProfileReadScope();
  const pending = useSessionDeletionPending(id);
  const deleted = useSessionDeletedByOperator(id);
  return useQuery(sessionScopedDetailOptions(id, params, { enabled: !pending && !deleted }));
}

/**
 * A session's id and lineage, sharing `useSession`'s cache entry without polling it or
 * re-rendering on the rest of its live detail.
 */
export function useSessionLineage(id: string, enabled = true) {
  const { params } = useProfileReadScope();
  const pending = useSessionDeletionPending(id);
  const deleted = useSessionDeletedByOperator(id);
  return useQuery({
    ...sessionScopedDetailOptions(id, params, {
      enabled: enabled && !pending && !deleted,
      liveTail: false,
    }),
    select: session => ({
      id: session.id,
      lineage: session.lineage && {
        kind: session.lineage.kind,
        parent_session_id: session.lineage.parent_session_id,
        root_session_id: session.lineage.root_session_id,
      },
    }),
  }).data;
}

export function useSessionById(id: string, workspace: string) {
  const { params } = useProfileReadScope();
  const pending = useSessionDeletionPending(id);
  const deleted = useSessionDeletedByOperator(id);
  return useQuery(
    sessionScopedDetailOptions(id, params, {
      enabled: workspace.trim() !== "" && !pending && !deleted,
    })
  );
}

export function useSessionGoal(workspaceId: string, sessionId: string, enabled = true) {
  return useQuery({
    ...sessionGoalOptions(workspaceId, sessionId),
    enabled: enabled && workspaceId !== "" && sessionId !== "",
  });
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
