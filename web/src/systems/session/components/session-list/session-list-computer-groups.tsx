import { useQueries } from "@tanstack/react-query";
import { use, useState } from "react";

import { isRemoteComputerId, LOCAL_COMPUTER_ID, type ComputerId } from "@/lib/computer-scope";
import { useProfileReadScope } from "@/systems/profiles";
import type { DataScope } from "@/systems/workspace";

import { fetchSessionCatalogPage } from "../../adapters/session-catalog-api";
import { SessionListGroupSelectionContext } from "../../contexts/session-list-group-selection-context-value";
import type { ComputerSessionsSource } from "../../hooks/use-computer-sessions-sources";
import { usePublishSessionListGroupRows } from "../../hooks/use-session-list-group-rows";
import type { SessionRowSelection } from "../../hooks/use-session-selection";
import { sessionCatalogFirstPageOptions } from "../../lib/session-catalog-options";
import { sessionCatalogScopedFilters } from "../../lib/session-catalog-scope";
import { SessionListComputerContext } from "../../lib/session-list-computer";
import {
  computerWorkspaceBadgeIdentity,
  matchesOfflineSessionQuery,
} from "../../lib/session-list-computer-filter";
import { sessionListSortParam } from "../../lib/session-list-preferences";
import { SessionListBadgeContext } from "../../lib/session-list-row-badge";
import { buildSessionListThreads, threadVisibleOrder } from "../../lib/session-list-threads";
import {
  buildTogetherComputerMap,
  buildTogetherProjectGroups,
  mergeTogetherFlatRows,
  selectNextTogetherComputer,
  type TogetherFlatPageEntry,
} from "../../lib/session-list-together";
import { sessionComputerOf, withSessionsComputer } from "../../lib/session-row-ref";
import type { SessionListFilters, SessionPayload } from "../../types";
import type { GroupRowProps } from "./session-list-group-body";
import { SessionListGroupSection } from "./session-list-groups";
import { SessionListRemoteFooter } from "./session-list-remote-body";
import { SessionListSkeleton } from "./session-list-skeleton";
import { SessionListThreadList } from "./session-list-thread-list";

interface TogetherExtraPage {
  sessions: SessionPayload[];
  hasMore: boolean;
  nextCursor: string | null;
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
  const { sources, view, emptyMessage } = props;
  const profile = useProfileReadScope();
  const groups = buildTogetherProjectGroups(sources, profile.params);
  const sourcesById = new Map(sources.map(source => [source.key, source]));
  const connecting = sources.filter(source => source.read === "connecting");
  const failed = sources.filter(source => source.read === "failed");

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
        failedComputers={failed.map(source => ({
          id: source.key,
          name: source.name,
          retry: source.retry,
        }))}
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
 * Together + None (`F6`, `O1`, `O2`, `S10`): one merged flat list across answered computers,
 * ordered by the window sort with remote computer labels and per-computer project badges.
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
  const profile = useProfileReadScope();
  const sortParam = sessionListSortParam(view.sort, {
    singlePair: false,
    shelved: view.shelf !== "active",
  });
  const baseFilters: SessionListFilters = { ...view.groupFilters, sort: sortParam };
  const filterSignature = JSON.stringify([scope, profile.key, baseFilters]);
  const [extraState, setExtraState] = useState<{
    key: string;
    byComputer: Record<string, TogetherExtraPage>;
  }>({ key: "", byComputer: {} });
  const [loadingMore, setLoadingMore] = useState(false);
  const extraByComputer = extraState.key === filterSignature ? extraState.byComputer : {};

  const answeredSources = sources.filter(
    source => source.read === "ok" || source.read === "offline"
  );
  const querySpecs = answeredSources.map(source => ({
    source,
    ...togetherComputerFilters(source, scope, baseFilters, profile.params),
  }));
  const queries = useQueries({
    queries: querySpecs.map(({ source, filters, emptyRemoteMatch }) => ({
      ...sessionCatalogFirstPageOptions(filters, source.key),
      enabled: source.read === "ok" && !emptyRemoteMatch && scope !== null,
    })),
  });

  const searchText = view.groupFilters.q ?? "";
  const entries: TogetherFlatPageEntry[] = querySpecs.map(({ source, emptyRemoteMatch }, index) => {
    const firstPage = emptyRemoteMatch ? undefined : queries[index]?.data;
    const extra = extraByComputer[source.key];
    const combined = [...(firstPage?.sessions ?? []), ...(extra?.sessions ?? [])];
    const sessions =
      source.read === "offline" && searchText.trim() !== ""
        ? combined.filter(session => matchesOfflineSessionQuery(session, searchText))
        : combined;
    return {
      source,
      sessions,
      hasMore: extra ? extra.hasMore : (firstPage?.page.has_more ?? false),
      nextCursor: extra ? extra.nextCursor : (firstPage?.page.next_cursor ?? null),
    };
  });

  const merged = mergeTogetherFlatRows(entries, view.sort);
  const mergedSessions = merged.map(row => row.session);
  const threads = buildSessionListThreads(mergedSessions, "");
  const groupSelection = use(SessionListGroupSelectionContext);
  usePublishSessionListGroupRows(groupSelection?.rows, "together-flat", {
    rank: "0000",
    sessions: mergedSessions,
    visibleOrder: threadVisibleOrder(threads, rows.collapsedThreads),
    pair: null,
  });
  const computerMap = buildTogetherComputerMap(entries);
  const namesByComputer = new Map<ComputerId, ReadonlyMap<string, string>>(
    answeredSources.map(source => [
      source.key,
      new Map(source.workspaces.map(workspace => [workspace.id, workspace.name])),
    ])
  );
  const badgeOf = view.allProjects
    ? (session: SessionPayload) => {
        const computerId = sessionComputerOf(session) ?? LOCAL_COMPUTER_ID;
        const names = namesByComputer.get(computerId) ?? new Map<string, string>();
        return computerWorkspaceBadgeIdentity(session.workspace_id, names, {
          isRemote: isRemoteComputerId(computerId),
        });
      }
    : undefined;

  const nextToLoad = selectNextTogetherComputer(entries);
  const handleLoadMore = async () => {
    if (!nextToLoad || !nextToLoad.nextCursor || loadingMore) return;
    const spec = querySpecs.find(item => item.source.key === nextToLoad.computerId);
    if (!spec) return;
    setLoadingMore(true);
    try {
      const page = await fetchSessionCatalogPage(
        { ...spec.filters, cursor: nextToLoad.nextCursor },
        undefined,
        nextToLoad.computerId
      );
      const stampedSessions = withSessionsComputer(page.sessions, nextToLoad.computerId);
      setExtraState(previous => {
        const base = previous.key === filterSignature ? previous.byComputer : {};
        const existing = base[nextToLoad.computerId];
        return {
          key: filterSignature,
          byComputer: {
            ...base,
            [nextToLoad.computerId]: {
              sessions: [...(existing?.sessions ?? []), ...stampedSessions],
              hasMore: page.page.has_more,
              nextCursor: page.page.next_cursor ?? null,
            },
          },
        };
      });
    } finally {
      setLoadingMore(false);
    }
  };

  const connecting = sources.filter(source => source.read === "connecting");
  const failed = sources.filter(source => source.read === "failed");
  const initialLoading =
    merged.length === 0 &&
    queries.some((query, index) => query.isLoading && querySpecs[index]?.source.read === "ok");
  if (initialLoading && connecting.length === 0) return <SessionListSkeleton />;

  return (
    <>
      <SessionListComputerContext value={computerMap}>
        <SessionListBadgeContext value={badgeOf}>
          <SessionListThreadList
            {...rows}
            threads={threads}
            ownerOf={view.aggregate ? view.ownerOf : undefined}
            onSelectSession={session => {
              const computerId = sessionComputerOf(session);
              rows.onSelectSession(
                isRemoteComputerId(computerId) ? { ...session, computerId } : session
              );
            }}
            selection={selection}
            emptyMessage={connecting.length > 0 || failed.length > 0 ? null : emptyMessage}
          />
        </SessionListBadgeContext>
      </SessionListComputerContext>
      <SessionListRemoteFooter
        failedComputers={failed.map(source => ({
          id: source.key,
          name: source.name,
          retry: source.retry,
        }))}
        hasMore={nextToLoad !== null}
        loadingComputers={connecting.map(source => ({ id: source.key, name: source.name }))}
        loadingMore={loadingMore}
        loadMoreLabel={nextToLoad ? `Load more from ${nextToLoad.computerName}` : undefined}
        onLoadMore={() => void handleLoadMore()}
        testIdPrefix={rows.testIdPrefix}
      />
    </>
  );
}
