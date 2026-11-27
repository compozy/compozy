import { use } from "react";

import { isRemoteComputerId, LOCAL_COMPUTER_ID, type ComputerId } from "@/lib/computer-scope";
import { useProfileReadScope } from "@/systems/profiles";
import type { DataScope } from "@/systems/workspace";

import { SessionListGroupSelectionContext } from "../../contexts/session-list-group-selection-context-value";
import { type ComputerSessionsSource } from "../../hooks/use-computer-sessions-sources";
import { usePublishSessionListGroupRows } from "../../hooks/use-session-list-group-rows";
import { useSessionListTogetherFlatPages } from "../../hooks/use-session-list-together-flat-pages";
import type { SessionRowSelection } from "../../hooks/use-session-selection";
import { SessionListComputerContext } from "../../lib/session-list-computer";
import { computerWorkspaceBadgeIdentity } from "../../lib/session-list-computer-filter";
import type { SessionListGroupSpec } from "../../lib/session-list-groups";
import { SessionListBadgeContext } from "../../lib/session-list-row-badge";
import { buildSessionListThreads, threadVisibleOrder } from "../../lib/session-list-threads";
import {
  buildTogetherComputerMap,
  buildTogetherProfileGroups,
  buildTogetherProjectGroups,
  mergeTogetherFlatRows,
} from "../../lib/session-list-together";
import { sessionComputerOf } from "../../lib/session-row-ref";
import type { SessionPayload } from "../../types";
import type { GroupRowProps } from "./session-list-group-body";
import { SessionListGroupSection } from "./session-list-groups";
import { SessionListRemoteFooter } from "./session-list-remote-body";
import { SessionListSkeleton } from "./session-list-skeleton";
import { SessionListThreadList } from "./session-list-thread-list";

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
  const failed = sources.flatMap(source =>
    source.read === "failed" ? [{ id: source.key, name: source.name, retry: source.retry }] : []
  );

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
  const {
    answeredSources,
    entries,
    initialLoading,
    loadingMore,
    nextToLoad,
    handleLoadMore,
    failed,
  } = useSessionListTogetherFlatPages({ sources, scope, view });

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

  const connecting = sources.filter(source => source.read === "connecting");

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
