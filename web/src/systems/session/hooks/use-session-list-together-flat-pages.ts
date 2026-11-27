import { QueryClientContext, useQueries, type QueryClient } from "@tanstack/react-query";
import { useContext, useState } from "react";

import { useProfileReadScope } from "@/systems/profiles";
import type { DataScope } from "@/systems/workspace";

import { sessionCatalogPageQueryOptions } from "../lib/session-catalog-options";
import { sessionCatalogScopedFilters } from "../lib/session-catalog-scope";
import { sessionListSortParam } from "../lib/session-list-preferences";
import { readCachedOfflineRowsForComputer } from "../lib/session-offline-catalog";
import {
  selectNextTogetherComputer,
  type TogetherFlatPageEntry,
} from "../lib/session-list-together";
import type { SessionCatalogPageResponse, SessionListFilters, SessionPayload } from "../types";
import type { SessionListViewModel } from "./session-list-view-types";
import type { ComputerSessionsSource } from "./use-computer-sessions-sources";

interface TogetherSourceSpec {
  source: ComputerSessionsSource;
  filters: SessionListFilters;
  emptyRemoteMatch: boolean;
}

interface TogetherPageSlot extends TogetherSourceSpec {
  sourceIndex: number;
  cursor: string | undefined;
}

function togetherComputerFilters(
  source: ComputerSessionsSource,
  scope: DataScope | null,
  baseFilters: SessionListFilters,
  localLens: ReturnType<typeof useProfileReadScope>["params"]
): { filters: SessionListFilters; emptyRemoteMatch: boolean } {
  if (!source.isLocal) {
    if (scope?.kind === "project") {
      const matched = source.matchedWorkspaceIds ?? [];
      if (matched.length === 0) {
        return {
          filters: { ...baseFilters, all_workspaces: true, all_profiles: true },
          emptyRemoteMatch: true,
        };
      }
      return {
        filters: {
          ...baseFilters,
          all_workspaces: true,
          workspace_ids: [...matched],
          all_profiles: true,
        },
        emptyRemoteMatch: false,
      };
    }
    return {
      filters: sessionCatalogScopedFilters(
        scope?.kind === "user" ? scope : { kind: "all" },
        baseFilters,
        { all_profiles: true }
      ),
      emptyRemoteMatch: false,
    };
  }
  return {
    filters: sessionCatalogScopedFilters(scope, baseFilters, localLens),
    emptyRemoteMatch: false,
  };
}

function buildTogetherPageSlots(
  specs: readonly TogetherSourceSpec[],
  extraCursors: Readonly<Record<string, readonly string[]>>
): TogetherPageSlot[] {
  const slots: TogetherPageSlot[] = [];
  for (let index = 0; index < specs.length; index++) {
    const spec = specs[index]!;
    slots.push({ sourceIndex: index, ...spec, cursor: undefined });
    for (const cursor of extraCursors[spec.source.key] ?? []) {
      slots.push({ sourceIndex: index, ...spec, cursor });
    }
  }
  return slots;
}

interface TogetherPageResult {
  data?: SessionCatalogPageResponse;
  isLoading: boolean;
  isError: boolean;
  refetch: () => Promise<unknown>;
}

function togetherFlatPages(
  querySpecs: readonly TogetherSourceSpec[],
  slots: readonly TogetherPageSlot[],
  queries: readonly TogetherPageResult[],
  queryClient: QueryClient | undefined,
  scope: DataScope | null
) {
  const buckets = querySpecs.map(spec => ({
    source: spec.source,
    sessions: [] as SessionPayload[],
    seen: new Set<string>(),
    hasMore: false,
    nextCursor: null as string | null,
    filters: spec.filters,
    skip: spec.emptyRemoteMatch,
  }));
  let loadingMore = false;
  let initialLoading = false;

  for (let index = 0; index < slots.length; index++) {
    const slot = slots[index]!;
    const bucket = buckets[slot.sourceIndex]!;
    const query = queries[index];
    if (slot.emptyRemoteMatch || slot.source.read !== "ok" || scope === null) continue;
    if (query?.isLoading) {
      if (slot.cursor) loadingMore = true;
      else initialLoading = true;
    }
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

  const entries: TogetherFlatPageEntry[] = buckets.map(bucket => {
    if (bucket.source.read === "offline" && !bucket.skip && scope !== null) {
      const offlineRows = readCachedOfflineRowsForComputer(
        queryClient,
        bucket.source.key,
        bucket.filters
      );
      return { source: bucket.source, sessions: offlineRows, hasMore: false, nextCursor: null };
    }
    return {
      source: bucket.source,
      sessions: bucket.sessions,
      hasMore: bucket.hasMore,
      nextCursor: bucket.nextCursor,
    };
  });

  return { entries, loadingMore, initialLoading };
}

function togetherFlatFailures(
  sources: readonly ComputerSessionsSource[],
  slots: readonly TogetherPageSlot[],
  queries: readonly TogetherPageResult[],
  scope: DataScope | null
) {
  return sources.flatMap(source => {
    const failedPages = slots.flatMap((slot, index) =>
      slot.source.key === source.key &&
      source.read === "ok" &&
      scope !== null &&
      !slot.emptyRemoteMatch &&
      queries[index]?.isError
        ? [queries[index]!]
        : []
    );
    if (failedPages.length === 0 && source.read !== "failed") return [];
    return [
      {
        id: source.key,
        name: source.name,
        retry:
          failedPages.length > 0
            ? () => {
                for (const page of failedPages) void page.refetch();
              }
            : source.retry,
      },
    ];
  });
}

/** Reads and advances each computer's catalog independently in the Together list. */
export function useSessionListTogetherFlatPages({
  sources,
  scope,
  view,
}: {
  sources: readonly ComputerSessionsSource[];
  scope: DataScope | null;
  view: Pick<SessionListViewModel, "sort" | "shelf" | "groupFilters">;
}) {
  const queryClient = useContext(QueryClientContext);
  const profile = useProfileReadScope();
  const sortParam = sessionListSortParam(view.sort, {
    singlePair: false,
    shelved: view.shelf !== "active",
  });
  const baseFilters: SessionListFilters = { ...view.groupFilters, sort: sortParam };
  const filterSignature = JSON.stringify([scope, profile.key, baseFilters]);
  const [cursorState, setCursorState] = useState<{
    key: string;
    byComputer: Record<string, string[]>;
  }>({ key: "", byComputer: {} });
  const extraCursors = cursorState.key === filterSignature ? cursorState.byComputer : {};

  const answeredSources = sources.filter(
    source => source.read === "ok" || source.read === "offline"
  );
  const querySpecs: TogetherSourceSpec[] = answeredSources.map(source => ({
    source,
    ...togetherComputerFilters(source, scope, baseFilters, profile.params),
  }));
  const slots = buildTogetherPageSlots(querySpecs, extraCursors);
  const queries = useQueries({
    queries: slots.map(slot => ({
      ...sessionCatalogPageQueryOptions(slot.filters, slot.cursor, slot.source.key),
      enabled: slot.source.read === "ok" && !slot.emptyRemoteMatch && scope !== null,
    })),
  });

  const { entries, loadingMore, initialLoading } = togetherFlatPages(
    querySpecs,
    slots,
    queries,
    queryClient,
    scope
  );

  const nextToLoad = selectNextTogetherComputer(entries, view.sort);
  const handleLoadMore = () => {
    if (!nextToLoad?.nextCursor || loadingMore) return;
    const continuation = slots.findIndex(
      slot => slot.source.key === nextToLoad.computerId && slot.cursor === nextToLoad.nextCursor
    );
    if (continuation !== -1 && queries[continuation]?.isError) {
      void queries[continuation]?.refetch();
      return;
    }
    setCursorState(previous => {
      const base = previous.key === filterSignature ? previous.byComputer : {};
      const existing = base[nextToLoad.computerId] ?? [];
      if (existing.includes(nextToLoad.nextCursor!)) return previous;
      return {
        key: filterSignature,
        byComputer: {
          ...base,
          [nextToLoad.computerId]: [...existing, nextToLoad.nextCursor!],
        },
      };
    });
  };

  const failed = togetherFlatFailures(sources, slots, queries, scope);
  return {
    answeredSources,
    entries,
    initialLoading,
    loadingMore,
    nextToLoad,
    handleLoadMore,
    failed,
  };
}
