import { QueryClientContext, useQueries, type QueryClient } from "@tanstack/react-query";
import { use, useContext, useState } from "react";

import { isRemoteComputerId, LOCAL_COMPUTER_ID, type ComputerId } from "@/lib/computer-scope";
import { useProfileReadScope } from "@/systems/profiles";
import type { DataScope } from "@/systems/workspace";

import { SessionListGroupSelectionContext } from "../../contexts/session-list-group-selection-context-value";
import {
  readCachedOfflineRowsForComputer,
  type ComputerSessionsSource,
} from "../../hooks/use-computer-sessions-sources";
import { usePublishSessionListGroupRows } from "../../hooks/use-session-list-group-rows";
import type { SessionRowSelection } from "../../hooks/use-session-selection";
import { sessionCatalogPageQueryOptions } from "../../lib/session-catalog-options";
import { sessionCatalogScopedFilters } from "../../lib/session-catalog-scope";
import { SessionListComputerContext } from "../../lib/session-list-computer";
import { computerWorkspaceBadgeIdentity } from "../../lib/session-list-computer-filter";
import type { SessionListGroupSpec } from "../../lib/session-list-groups";
import { sessionListSortParam } from "../../lib/session-list-preferences";
import { SessionListBadgeContext } from "../../lib/session-list-row-badge";
import { buildSessionListThreads, threadVisibleOrder } from "../../lib/session-list-threads";
import {
  buildTogetherComputerMap,
  buildTogetherProfileGroups,
  buildTogetherProjectGroups,
  mergeTogetherFlatRows,
  selectNextTogetherComputer,
  type TogetherFlatPageEntry,
} from "../../lib/session-list-together";
import { sessionComputerOf } from "../../lib/session-row-ref";
import type { SessionCatalogPageResponse, SessionListFilters, SessionPayload } from "../../types";
import type { GroupRowProps } from "./session-list-group-body";
import { SessionListGroupSection } from "./session-list-groups";
import { SessionListRemoteFooter } from "./session-list-remote-body";
import { SessionListSkeleton } from "./session-list-skeleton";
import { SessionListThreadList } from "./session-list-thread-list";

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

function TogetherGroupList({
  groups,
  sources,
  emptyMessage,
  ...props
}: GroupRowProps & {
  groups: readonly SessionListGroupSpec[];
  sources: readonly ComputerSessionsSource[];
  emptyMessage: string | null;
}) {
  const { view } = props;
  const sourcesById = new Map(sources.map(source => [source.key, source]));
  const connecting = sources.filter(source => source.read === "connecting");
  const failed = sources.flatMap(source => {
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

  return (
    <div className="flex flex-col gap-1" data-testid={`${props.testIdPrefix}-together-groups`}>
      {groups.map((group, index) => {
        const source = sourcesById.get(group.computerId ?? LOCAL_COMPUTER_ID);
        const groupView = source
          ? {
              ...view,
              computerId: source.key,
              workspaces: source.workspaces,
              matchedWorkspaceIds: source.matchedWorkspaceIds,
              readOnly: !source.canOperate,
            }
          : view;
        return (
          <SessionListGroupSection
            key={group.key}
            {...props}
            view={groupView}
            group={group}
            path={[index]}
            readOnly={source ? !source.canOperate : undefined}
          />
        );
      })}
      {groups.length === 0 && connecting.length === 0 && failed.length === 0 && emptyMessage ? (
        <p className="px-3 py-8 text-center text-small-body text-muted-foreground">
          {emptyMessage}
        </p>
      ) : null}
      <SessionListRemoteFooter
        failedComputers={failed}
        hasMore={false}
        loadingComputers={connecting.map(source => ({ id: source.key, name: source.name }))}
        loadingMore={false}
        onLoadMore={() => undefined}
        testIdPrefix={props.testIdPrefix}
      />
    </div>
  );
}

/**
 * Together + Project (`F6`, `N3`): one group per `(computer, project)` built from per-computer
 * facets. Each header carries the computer's mark in the chevron cell at rest and names the
 * computer after the project. Failed and connecting computers sit inline below.
 */
export function SessionListTogetherProjectGroups(
  props: GroupRowProps & {
    sources: readonly ComputerSessionsSource[];
    emptyMessage: string | null;
  }
) {
  const profile = useProfileReadScope();
  const groups = buildTogetherProjectGroups(props.sources, profile.params);
  return <TogetherGroupList {...props} groups={groups} />;
}

/**
 * Together + Profile (`Opus 3`, `Astra 9`): one group per `(computer, profile)` built from
 * per-computer facets, with computer marks and names in each header.
 */
export function SessionListTogetherProfileGroups(
  props: GroupRowProps & {
    sources: readonly ComputerSessionsSource[];
    scope: DataScope | null;
    emptyMessage: string | null;
  }
) {
  const { scope, ...rest } = props;
  const groups = buildTogetherProfileGroups(props.sources, scope ?? { kind: "all" });
  return <TogetherGroupList {...rest} groups={groups} />;
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

function togetherBadgeResolver(answeredSources: readonly ComputerSessionsSource[]) {
  const namesByComputer = new Map<ComputerId, ReadonlyMap<string, string>>(
    answeredSources.map(source => [
      source.key,
      new Map(source.workspaces.map(workspace => [workspace.id, workspace.name])),
    ])
  );
  return (session: SessionPayload) => {
    const computerId = sessionComputerOf(session) ?? LOCAL_COMPUTER_ID;
    const names = namesByComputer.get(computerId) ?? new Map<string, string>();
    return computerWorkspaceBadgeIdentity(session.workspace_id, names, {
      isRemote: isRemoteComputerId(computerId),
    });
  };
}

/**
 * Together + None (`F6`, `O1`, `O2`, `S10`): one merged flat list across answered computers,
 * ordered by the window sort with remote computer labels and per-computer project badges.
 * Continuation pages live in computer-scoped paged TanStack queries so invalidations and
 * 15s polling cover every loaded page (`Opus 7`, `Astra 14`).
 */
export function SessionListTogetherFlatBody({
  sources,
  scope,
  view,
  selection,
  emptyMessage,
  ...rows
}: Omit<GroupRowProps, "onNewSession"> & {
  sources: readonly ComputerSessionsSource[];
  scope: DataScope | null;
  selection?: SessionRowSelection;
  emptyMessage: string | null;
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

  const merged = mergeTogetherFlatRows(entries, view.sort);
  const mergedSessions = merged.map(row => row.session);
  const writable = new Set(sources.flatMap(source => (source.canOperate ? [source.key] : [])));
  const selectable = merged.flatMap(row => (writable.has(row.computerId) ? [row.session] : []));
  const selectionThreads = buildSessionListThreads(selectable, "");
  const threads = buildSessionListThreads(mergedSessions, "");
  const groupSelection = use(SessionListGroupSelectionContext);
  usePublishSessionListGroupRows(groupSelection?.rows, "together-flat", {
    rank: "0000",
    sessions: selectable,
    visibleOrder: threadVisibleOrder(selectionThreads, rows.collapsedThreads),
    pair: null,
  });

  const computerMap = buildTogetherComputerMap(entries);
  const badgeOf = view.allProjects ? togetherBadgeResolver(answeredSources) : undefined;

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

  const connecting = sources.filter(source => source.read === "connecting");
  const failed = togetherFlatFailures(sources, slots, queries, scope);

  if (initialLoading && merged.length === 0) return <SessionListSkeleton />;

  return (
    <div className="flex flex-col gap-0.5" data-testid={`${rows.testIdPrefix}-together-flat`}>
      <SessionListComputerContext value={computerMap}>
        <SessionListBadgeContext value={badgeOf}>
          <SessionListThreadList
            collapsedThreads={rows.collapsedThreads}
            currentSessionId={rows.currentSessionId}
            emptyMessage={connecting.length === 0 && failed.length === 0 ? emptyMessage : null}
            onSelectSession={rows.onSelectSession}
            onThreadBranchChange={rows.onThreadBranchChange}
            ownerOf={view.aggregate ? view.ownerOf : undefined}
            selection={selection}
            sessionActions={rows.sessionActions}
            subagentThreads={rows.subagentThreads}
            testIdPrefix={rows.testIdPrefix}
            threads={threads}
          />
        </SessionListBadgeContext>
      </SessionListComputerContext>
      <SessionListRemoteFooter
        failedComputers={failed}
        hasMore={nextToLoad !== null}
        loadingComputers={connecting.map(source => ({ id: source.key, name: source.name }))}
        loadingMore={loadingMore}
        loadMoreLabel={nextToLoad ? `Load more from ${nextToLoad.computerName}` : undefined}
        onLoadMore={handleLoadMore}
        testIdPrefix={rows.testIdPrefix}
      />
    </div>
  );
}
