import { QueryClient, QueryClientContext, useQueries } from "@tanstack/react-query";
import { useContext } from "react";

import { isRemoteComputerId, LOCAL_COMPUTER_ID, type ComputerId } from "@/lib/computer-scope";
import type { ProfileScopeParams } from "@/systems/profiles";
import type { DataScope, WorkspaceBadgeIdentity } from "@/systems/workspace";

import { sessionCatalogFirstPageOptions } from "../lib/session-catalog-options";
import { sessionCatalogScopedFilters } from "../lib/session-catalog-scope";
import type { SessionRowComputerInfo } from "../lib/session-list-computer";
import {
  computerWorkspaceBadgeIdentity,
  matchesOfflineSessionQuery,
} from "../lib/session-list-computer-filter";
import { sessionListPinnedFilters } from "../lib/session-list-pinned";
import { sessionComputerOf, sessionRowKey } from "../lib/session-row-ref";
import type { SessionListFilters, SessionPayload } from "../types";
import type { ComputerSessionsSource } from "./use-computer-sessions-sources";
import type { SessionListPinnedRead } from "./use-session-list-pinned-read";

const NOOP_QUERY_CLIENT = new QueryClient({
  defaultOptions: { queries: { enabled: false, retry: false } },
});

export interface MultiComputerPinnedRead extends SessionListPinnedRead {
  /** Remote-computer provenance for every remote pin (`Q8`, `R10`). */
  computerBySessionId?: ReadonlyMap<string, SessionRowComputerInfo>;
  /** Per-computer project badge resolver (`P1`). */
  badgeOf?: (session: SessionPayload) => WorkspaceBadgeIdentity;
}

/**
 * One combined Pinned section on top across all computers in view (`Q8`, `R10`, `O6`).
 * Remote pins carry their computer label; reorder is omitted because multiple computers
 * never share one reorderable pin list.
 */
export function useMultiComputerPinned(options: {
  enabled: boolean;
  sources: readonly ComputerSessionsSource[];
  scope: DataScope | null;
  localLens: ProfileScopeParams;
  groupFilters: SessionListFilters;
  allProjects: boolean;
}): MultiComputerPinnedRead | undefined {
  const contextClient = useContext(QueryClientContext);
  const { enabled, sources, scope, localLens, groupFilters, allProjects } = options;
  const active = enabled && contextClient !== undefined && scope !== null;
  const pinnedBase = sessionListPinnedFilters(groupFilters);
  const answered = active
    ? sources.filter(source => source.read === "ok" || source.read === "offline")
    : [];

  const specs = answered.map(source => {
    if (source.isLocal) {
      return {
        source,
        filters: sessionCatalogScopedFilters(scope, pinnedBase, localLens),
        skip: false,
      };
    }
    if (scope?.kind === "project") {
      const matched = source.matchedWorkspaceIds ?? [];
      if (matched.length === 0) {
        return {
          source,
          filters: { ...pinnedBase, all_workspaces: true, all_profiles: true },
          skip: true,
        };
      }
      return {
        source,
        filters: {
          ...pinnedBase,
          all_workspaces: true,
          workspace_ids: [...matched],
          all_profiles: true,
        },
        skip: false,
      };
    }
    return {
      source,
      filters: sessionCatalogScopedFilters(
        scope?.kind === "user" ? scope : { kind: "all" },
        pinnedBase,
        { all_profiles: true }
      ),
      skip: false,
    };
  });

  const queries = useQueries(
    {
      queries: specs.map(({ source, filters, skip }) => ({
        ...sessionCatalogFirstPageOptions(filters, source.key),
        enabled: active && source.read === "ok" && !skip,
      })),
    },
    contextClient ?? NOOP_QUERY_CLIENT
  );

  if (!active) return undefined;

  const search = groupFilters.q?.trim() ?? "";
  const sessions: SessionPayload[] = [];
  const computerBySessionId = new Map<string, SessionRowComputerInfo>();
  const namesByComputer = new Map<ComputerId, ReadonlyMap<string, string>>();
  let more = false;
  let loading = false;
  let allFailed = specs.length > 0;

  for (let index = 0; index < specs.length; index++) {
    const spec = specs[index]!;
    const query = queries[index];
    if (spec.skip) {
      allFailed = false;
      continue;
    }
    if (query?.isLoading && spec.source.read === "ok") loading = true;
    if (!query?.isError) allFailed = false;
    const page = query?.data;
    if (!page) continue;
    if (page.page.has_more) more = true;
    namesByComputer.set(
      spec.source.key,
      new Map(spec.source.workspaces.map(workspace => [workspace.id, workspace.name]))
    );
    const rows =
      spec.source.read === "offline" && search !== ""
        ? page.sessions.filter(session => matchesOfflineSessionQuery(session, search))
        : page.sessions;
    for (const session of rows) {
      sessions.push(session);
      if (!spec.source.isLocal) {
        computerBySessionId.set(
          sessionRowKey({ computerId: spec.source.key, sessionId: session.id }),
          {
            id: spec.source.key,
            name: spec.source.name,
            platform: spec.source.computer.platform,
            iconKind: spec.source.computer.iconKind,
            offline: spec.source.read === "offline",
          }
        );
      }
    }
  }

  const badgeOf = allProjects
    ? (session: SessionPayload) => {
        const computerId = sessionComputerOf(session) ?? LOCAL_COMPUTER_ID;
        const names = namesByComputer.get(computerId) ?? new Map<string, string>();
        return computerWorkspaceBadgeIdentity(session.workspace_id, names, {
          isRemote: isRemoteComputerId(computerId),
        });
      }
    : undefined;

  return {
    sessions,
    loading: loading && sessions.length === 0,
    failed: allFailed && sessions.length === 0,
    more,
    retry: () => {
      for (const query of queries) void query.refetch();
    },
    // Multi-computer Pinned never reorders across computers (`O6`).
    reorder: undefined,
    computerBySessionId,
    badgeOf,
  };
}
