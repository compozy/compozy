import { readCachedOfflineRowsForComputer } from "../lib/session-offline-catalog";
import {
  QueryClient,
  QueryClientContext,
  useQueries,
  type UseQueryResult,
} from "@tanstack/react-query";
import { useContext, useState } from "react";

import { isRemoteComputerId, type ComputerId } from "@/lib/computer-scope";
import type { ProfileScopeParams } from "@/systems/profiles";
import type { DataScope, WorkspaceBadgeIdentity } from "@/systems/workspace";

import { sessionCatalogPageQueryOptions } from "../lib/session-catalog-options";
import { sessionCatalogScopedFilters } from "../lib/session-catalog-scope";
import type { SessionRowComputerInfo } from "../lib/session-list-computer";
import { computerWorkspaceBadgeIdentity } from "../lib/session-list-computer-filter";
import { sessionListPair } from "../lib/session-list-groups";
import { sessionListPinnedFilters } from "../lib/session-list-pinned";
import { sessionListSortParam, type SessionListSort } from "../lib/session-list-preferences";
import { mergeTogetherFlatRows, selectNextTogetherComputer } from "../lib/session-list-together";
import { sessionComputerOf, sessionRowKey } from "../lib/session-row-ref";
import type { SessionCatalogPageResponse, SessionListFilters, SessionPayload } from "../types";
import { type ComputerSessionsSource } from "./use-computer-sessions-sources";
import type { SessionListPinnedRead } from "./use-session-list-pinned-read";
import { useSessionPinnedOrder } from "./use-session-pinned-order";

const NOOP_QUERY_CLIENT = new QueryClient({
  defaultOptions: { queries: { enabled: false, retry: false } },
});

export interface MultiComputerPinnedRead extends SessionListPinnedRead {
  /** Remote-computer provenance for every remote pin (`Q8`, `R10`). */
  computerBySessionId?: ReadonlyMap<string, SessionRowComputerInfo>;
  /** Per-computer project badge resolver (`P1`). */
  badgeOf?: (session: SessionPayload) => WorkspaceBadgeIdentity;
}

interface PinnedSourceSpec {
  source: ComputerSessionsSource;
  filters: SessionListFilters;
  skip: boolean;
}

interface PinnedPageSlot {
  sourceIndex: number;
  source: ComputerSessionsSource;
  filters: SessionListFilters;
  cursor: string | undefined;
  skip: boolean;
}

function buildPinnedSourceSpec(
  source: ComputerSessionsSource,
  scope: DataScope | null,
  pinnedBase: SessionListFilters,
  localLens: ProfileScopeParams
): PinnedSourceSpec {
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
}

function buildPinnedPageSlots(
  specs: readonly PinnedSourceSpec[],
  extraCursors: Readonly<Record<string, readonly string[]>>
): PinnedPageSlot[] {
  const slots: PinnedPageSlot[] = [];
  for (let index = 0; index < specs.length; index++) {
    const spec = specs[index]!;
    slots.push({ sourceIndex: index, ...spec, cursor: undefined });
    for (const cursor of extraCursors[spec.source.key] ?? []) {
      slots.push({ sourceIndex: index, ...spec, cursor });
    }
  }
  return slots;
}

function aggregatePinnedEntries(input: {
  specs: readonly PinnedSourceSpec[];
  slots: readonly PinnedPageSlot[];
  queries: readonly UseQueryResult<SessionCatalogPageResponse>[];
  queryClient: QueryClient;
  sort: SessionListSort;
}) {
  const { specs, slots, queries, queryClient, sort } = input;
  const computerBySessionId = new Map<string, SessionRowComputerInfo>();
  const namesByComputer = new Map<ComputerId, ReadonlyMap<string, string>>();
  const bySource = specs.map(spec => ({
    source: spec.source,
    sessions: [] as SessionPayload[],
    seen: new Set<string>(),
    hasMore: false,
    nextCursor: null as string | null,
    skip: spec.skip,
  }));

  let loading = false;
  let paging = false;
  let allFailed = specs.length > 0;
  let anyFailed = false;

  for (let index = 0; index < slots.length; index++) {
    const slot = slots[index]!;
    const bucket = bySource[slot.sourceIndex]!;
    const query = queries[index];
    if (slot.skip) {
      allFailed = false;
      continue;
    }
    if (slot.source.read === "offline") {
      allFailed = false;
      continue;
    }
    if (query?.isLoading) {
      if (slot.cursor) paging = true;
      else loading = true;
    }
    if (query?.isError) anyFailed = true;
    else allFailed = false;
    const page = query?.data;
    if (!page) continue;
    for (const session of page.sessions) {
      if (bucket.seen.has(session.id)) continue;
      bucket.seen.add(session.id);
      bucket.sessions.push(session);
    }
    bucket.hasMore = page.page.has_more;
    bucket.nextCursor = page.page.next_cursor ?? null;
  }

  for (const [index, bucket] of bySource.entries()) {
    namesByComputer.set(
      bucket.source.key,
      new Map(bucket.source.workspaces.map(workspace => [workspace.id, workspace.name]))
    );
    if (bucket.source.read === "offline" && !bucket.skip) {
      const offlinePins = readCachedOfflineRowsForComputer(
        queryClient,
        bucket.source.key,
        specs[index]!.filters
      );
      bucket.sessions.push(...offlinePins);
    }
    if (!bucket.source.isLocal) {
      const info: SessionRowComputerInfo = {
        id: bucket.source.key,
        name: bucket.source.name,
        platform: bucket.source.computer.platform,
        iconKind: bucket.source.computer.iconKind,
        offline: bucket.source.read === "offline",
        canOperate: bucket.source.canOperate,
      };
      for (const session of bucket.sessions) {
        computerBySessionId.set(
          sessionRowKey({ computerId: bucket.source.key, sessionId: session.id }),
          info
        );
      }
    }
  }

  const entries = bySource.map(({ source, sessions, hasMore, nextCursor }) => ({
    source,
    sessions,
    hasMore,
    nextCursor,
  }));
  const sessions =
    bySource.length > 1
      ? mergeTogetherFlatRows(entries, sort).map(row => row.session)
      : (bySource[0]?.sessions ?? []);
  const nextToLoad = selectNextTogetherComputer(entries, sort);

  return {
    sessions,
    loading,
    paging,
    failed: allFailed && sessions.length === 0,
    anyFailed,
    more: nextToLoad !== null,
    nextToLoad,
    computerBySessionId,
    namesByComputer,
  };
}

function pinnedBadgeResolver(
  namesByComputer: ReadonlyMap<ComputerId, ReadonlyMap<string, string>>
) {
  return (session: SessionPayload) => {
    const computerId = sessionComputerOf(session);
    const names = namesByComputer.get(computerId) ?? new Map<string, string>();
    return computerWorkspaceBadgeIdentity(session.workspace_id, names, {
      isRemote: isRemoteComputerId(computerId),
    });
  };
}

function answeredPinnedSources(sources: readonly ComputerSessionsSource[], active: boolean) {
  return active ? sources.filter(source => source.read === "ok" || source.read === "offline") : [];
}

function pinnedReorderPair(
  sources: readonly ComputerSessionsSource[],
  scope: DataScope | null,
  lens: ProfileScopeParams
) {
  const source = sources.length === 1 ? sources[0] : undefined;
  if (!source?.canOperate || source.read !== "ok") return null;
  if (source.isLocal) return sessionListPair(scope, lens, source.key);
  const [owner] = source.profiles ?? [];
  if (source.profiles?.length !== 1 || !owner || owner.archived) return null;
  if (scope?.kind === "user") return sessionListPair(scope, { profile: owner.name }, source.key);
  const [workspaceId] = source.matchedWorkspaceIds ?? [];
  if (scope?.kind !== "project" || source.matchedWorkspaceIds?.length !== 1 || !workspaceId)
    return null;
  return sessionListPair({ kind: "project", workspaceId }, { profile: owner.name }, source.key);
}

/**
 * One combined Pinned section on top across all computers in view (`Q8`, `R10`, `O6`).
 * Remote pins carry their computer label; pages advance per computer via cursor queries
 * with 15s polling; multi-computer Pinned sorts by the window sort; single-computer remote
 * scope allows reordering pins when a single `(computer, workspace, profile)` pair applies.
 */
export function useMultiComputerPinned(options: {
  enabled: boolean;
  sources: readonly ComputerSessionsSource[];
  scope: DataScope | null;
  localLens: ProfileScopeParams;
  groupFilters: SessionListFilters;
  allProjects: boolean;
  sort?: SessionListSort;
}): MultiComputerPinnedRead | undefined {
  const contextClient = useContext(QueryClientContext);
  const queryClient = contextClient ?? NOOP_QUERY_CLIENT;
  const {
    enabled,
    sources,
    scope,
    localLens,
    groupFilters,
    allProjects,
    sort = "last_activity",
  } = options;
  const active = enabled && contextClient !== undefined && scope !== null;
  const pinnedBase = sessionListPinnedFilters(groupFilters);
  if (sources.length > 1) pinnedBase.sort = sessionListSortParam(sort, { singlePair: false });
  const answered = answeredPinnedSources(sources, active);

  const filterKey = JSON.stringify([scope, localLens, pinnedBase]);
  const [cursorState, setCursorState] = useState<{
    key: string;
    byComputer: Record<string, string[]>;
  }>({ key: "", byComputer: {} });
  const extraCursors = cursorState.key === filterKey ? cursorState.byComputer : {};

  const specs = answered.map(source => buildPinnedSourceSpec(source, scope, pinnedBase, localLens));
  const slots = buildPinnedPageSlots(specs, extraCursors);
  const queries = useQueries(
    {
      queries: slots.map(slot => ({
        ...sessionCatalogPageQueryOptions(slot.filters, slot.cursor, slot.source.key),
        enabled: active && slot.source.read === "ok" && !slot.skip,
      })),
    },
    queryClient
  );

  // O6: single-computer scope may reorder pins when one (computer, workspace, profile) pair applies.
  const reorder = useSessionPinnedOrder(pinnedReorderPair(answered, scope, localLens));

  if (!active) return undefined;

  const aggregated = aggregatePinnedEntries({
    specs,
    slots,
    queries,
    queryClient,
    sort,
  });

  const retryAll = () => {
    slots.forEach((slot, index) => {
      if (!slot.skip && slot.source.read === "ok") void queries[index]?.refetch();
    });
  };
  const loadMore = () => {
    const target = aggregated.nextToLoad;
    if (!target?.nextCursor || aggregated.paging) return;
    setCursorState(previous => {
      const base = previous.key === filterKey ? previous.byComputer : {};
      const existing = base[target.computerId] ?? [];
      if (existing.includes(target.nextCursor!)) return previous;
      return {
        key: filterKey,
        byComputer: { ...base, [target.computerId]: [...existing, target.nextCursor!] },
      };
    });
  };

  const badgeOf = allProjects ? pinnedBadgeResolver(aggregated.namesByComputer) : undefined;

  return {
    sessions: aggregated.sessions,
    loading: aggregated.loading && aggregated.sessions.length === 0,
    failed: aggregated.failed,
    more: aggregated.more,
    retry: retryAll,
    reorder:
      aggregated.more || aggregated.anyFailed || aggregated.loading || groupFilters.q?.trim()
        ? undefined
        : reorder,
    catalog: {
      sessions: aggregated.sessions,
      paging: aggregated.paging,
      previous: false,
      next: aggregated.more,
      failed: aggregated.anyFailed,
      previousPage: () => undefined,
      nextPage: loadMore,
      retry: retryAll,
    },
    computerBySessionId: aggregated.computerBySessionId,
    badgeOf,
  };
}
