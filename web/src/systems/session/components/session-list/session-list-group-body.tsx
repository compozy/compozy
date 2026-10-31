import type { SessionThreadBranchChange } from "../../lib/session-thread-branch";
import { use } from "react";

import { TriangleAlert } from "lucide-react";

import { Button, Icon } from "@compozy/ui";

import { emptyForScope } from "@/systems/profiles";
import { useActiveWorkspace } from "@/systems/workspace";

import { SessionListGroupSelectionContext } from "../../contexts/session-list-group-selection-context-value";
import { useSessionCatalog } from "../../hooks/use-session-catalog";
import { useSessionFolders } from "../../hooks/use-session-folders";
import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import { useSessionListFolderLayout } from "../../hooks/use-session-list-folder-layout";
import { usePublishSessionListGroupRows } from "../../hooks/use-session-list-group-rows";
import { useSessionListGroups } from "../../hooks/use-session-list-groups";
import type { SessionListViewModel } from "../../hooks/use-session-list-view";
import type { SessionCreateOpenOptions } from "../../lib/session-create-target";
import { sessionListGroupRank, sessionListPairKey } from "../../lib/session-list-group-rows";
import { sessionListGroupFilters, type SessionListGroupSpec } from "../../lib/session-list-groups";
import { buildSessionListThreads, threadVisibleOrder } from "../../lib/session-list-threads";
import { sessionListShelfNoun } from "../../lib/session-list-shelf";
import type { SessionListTarget } from "../../types";
import { SessionFolderDialogs } from "./session-folder-dialogs";
import { SessionListSections } from "./session-list-sections";
import { SessionGroupRowsSkeleton } from "./session-list-skeleton";
import { SessionListThreadList } from "./session-list-thread-list";
import type { SessionSelectionDrag } from "../../hooks/use-session-list-dnd";
import type { SessionListPair } from "../../lib/session-list-groups";

export interface GroupRowProps {
  view: SessionListViewModel;
  filtering: boolean;
  collapsedThreads: ReadonlySet<string>;
  subagentThreads: ReadonlySet<string>;
  currentSessionId?: string;
  onThreadBranchChange: SessionThreadBranchChange;
  onSelectSession: (session: SessionListTarget) => void;
  /** Opens creation; a group names its folder and the pair that folder belongs to. */
  onNewSession: (options?: SessionCreateOpenOptions) => void;
  sessionActions: SessionLifecycleActionHandlers;
  testIdPrefix: string;
}

export function GroupNote({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-6 items-center gap-2 px-3 py-1 text-micro text-subtle">
      {children}
    </div>
  );
}

export function GroupLoadFailure({
  retryTestId,
  onRetry,
}: {
  retryTestId: string;
  onRetry: () => void;
}) {
  return (
    <GroupNote>
      <Icon as={TriangleAlert} size="sm" className="text-warning" />
      Couldn&rsquo;t load sessions
      <Button
        variant="ghost"
        size="sm"
        className="ml-auto"
        data-testid={retryTestId}
        onClick={onRetry}
      >
        Retry
      </Button>
    </GroupNote>
  );
}

/**
 * One group's body. A group whose query is one (workspace, profile) pair reads
 * its own page with that pair's folders and manual order (BR-11). A group that
 * spans pairs — a project under All profiles, or a profile under All projects —
 * splits along the other axis into groups that each are one pair.
 */
export type SessionListSubgroupRenderer = (
  subgroup: SessionListGroupSpec,
  path: readonly number[]
) => React.ReactNode;

export function SessionListGroupBody(
  props: GroupRowProps & {
    group: SessionListGroupSpec;
    path: readonly number[];
    nested?: boolean;
    /** Renders one split subgroup with its own header (owned by the group list). */
    renderSubgroup: SessionListSubgroupRenderer;
  }
) {
  return props.group.pair ? (
    <SessionListPairGroupBody {...props} />
  ) : (
    <SessionListGroupSplit {...props} />
  );
}

function SessionListGroupSplit({
  group,
  path,
  renderSubgroup,
  ...props
}: GroupRowProps & {
  group: SessionListGroupSpec;
  path: readonly number[];
  renderSubgroup: SessionListSubgroupRenderer;
}) {
  const { view } = props;
  const { registeredWorkspaces: workspaces } = useActiveWorkspace();
  const split = useSessionListGroups({
    groupBy: group.kind === "project" ? "profile" : "project",
    scope: group.scope,
    lens: group.lens,
    workspaces,
    shelf: view.shelf,
    search: view.groupFilters.q,
    enabled: true,
  });
  if (split.error) {
    return (
      <GroupLoadFailure
        retryTestId={`${props.testIdPrefix}-group-${group.key}-retry`}
        onRetry={split.error.retry}
      />
    );
  }
  if (split.loading) return <SessionGroupRowsSkeleton />;
  const subgroups = split.groups.map(subgroup => ({
    ...subgroup,
    key: `${group.key}/${subgroup.key}`,
  }));
  const only = subgroups.length === 1 ? subgroups[0] : undefined;
  // One pair needs no second header: its rows render as the group's own, and
  // keep the tags the group's header does not settle.
  if (only) {
    return (
      <SessionListPairGroupBody
        {...props}
        group={{ ...only, kind: group.kind }}
        path={[...path, 0]}
      />
    );
  }
  if (subgroups.length === 0) {
    const empty = emptyForScope(sessionListShelfNoun(view.shelf), view.scopeLabel);
    return <GroupNote>{`${empty}.`}</GroupNote>;
  }
  return (
    <div
      className="flex flex-col gap-0.5 pl-2.5"
      data-testid={`${props.testIdPrefix}-split-${group.key}`}
    >
      {subgroups.map((subgroup, index) => renderSubgroup(subgroup, [...path, index]))}
    </div>
  );
}

function SessionListPairGroupBody({
  group,
  path,
  nested = false,
  ...props
}: GroupRowProps & { group: SessionListGroupSpec; path: readonly number[]; nested?: boolean }) {
  const { view } = props;
  const groupSelection = use(SessionListGroupSelectionContext);
  const catalog = useSessionCatalog(group.scope, sessionListGroupFilters(view, group), true, {
    facets: false,
    lens: group.lens,
  });
  const folders = useSessionFolders({
    workspaceId: group.pair?.workspaceId ?? null,
    profile: group.pair?.profile ?? "",
    enabled: group.pair !== null,
  });
  const threads = buildSessionListThreads(catalog.sessions, "");
  const layout = useSessionListFolderLayout(
    {
      layout: group.pair ? folders.layout : undefined,
      sort: view.sort,
      shelf: view.shelf,
      worktreeId: view.worktreeId,
      catalog,
    },
    threads,
    props.filtering
  );
  usePublishSessionListGroupRows(groupSelection?.rows, group.key, {
    rank: sessionListGroupRank(path),
    sessions: catalog.sessions,
    visibleOrder: threadVisibleOrder(layout.visibleThreads, props.collapsedThreads),
    pair: group.pair,
  });
  const select = (session: SessionListTarget) => {
    if (!catalog.paging) props.onSelectSession(session);
  };
  const pair = group.pair;
  const newSessionInFolder = (folderId: string) =>
    props.onNewSession(
      pair ? { folderId, workspaceId: pair.workspaceId, profile: pair.profile } : { folderId }
    );
  const drag = groupSelection?.drag;
  // A dragged selection moves inside one pair's layout; other groups never take its drop.
  const selectionDrag = selectionDragForPair(drag, pair);
  const empty = emptyForScope(sessionListShelfNoun(view.shelf), view.scopeLabel);
  // The grouped dimension is the header: a profile group repeats no owner tag.
  // A split subgroup's own header and its parent's name both axes already.
  const ownerOf = !nested && group.kind === "project" && view.aggregate ? view.ownerOf : undefined;

  if (catalog.failed && catalog.sessions.length === 0) {
    // One unreachable group never blanks the list: it states what happened in
    // its own place and offers the one useful action.
    return (
      <GroupLoadFailure
        retryTestId={`${props.testIdPrefix}-group-${group.key}-retry`}
        onRetry={catalog.retry}
      />
    );
  }
  if (catalog.loading) return <SessionGroupRowsSkeleton />;
  return (
    <div className="flex flex-col gap-0.5 pl-2.5">
      {layout.layout && layout.sections ? (
        <>
          <SessionListSections
            ownerOf={ownerOf}
            sections={layout.sections}
            layout={layout.layout}
            dnd={layout.dnd}
            selectionDrag={selectionDrag}
            orderable={layout.orderable}
            filtering={props.filtering}
            collapsedThreads={props.collapsedThreads}
            subagentThreads={props.subagentThreads}
            collapsedFolders={layout.collapsedFolders}
            currentSessionId={props.currentSessionId}
            onThreadBranchChange={props.onThreadBranchChange}
            onToggleFolder={layout.toggleFolder}
            onSelectSession={select}
            onNewSessionInFolder={newSessionInFolder}
            onFolderDialog={layout.setDialog}
            selection={groupSelection?.selection}
            sessionActions={props.sessionActions}
            testIdPrefix={props.testIdPrefix}
            emptyMessage={`${empty}.`}
          />
          <SessionFolderDialogs
            state={layout.dialog}
            onClose={() => layout.setDialog(null)}
            onCreate={layout.layout.createFolder}
            onRename={layout.layout.renameFolder}
            onDelete={layout.layout.deleteFolder}
          />
        </>
      ) : (
        <GroupThreads
          {...props}
          threads={threads}
          emptyMessage={`${empty}.`}
          ownerOf={ownerOf}
          onSelectSession={select}
        />
      )}
      <GroupPager catalog={catalog} />
    </div>
  );
}

type GroupCatalog = ReturnType<typeof useSessionCatalog>;

function GroupThreads({
  threads,
  emptyMessage,
  ownerOf,
  onSelectSession,
  ...props
}: Omit<GroupRowProps, "onSelectSession"> & {
  threads: ReturnType<typeof buildSessionListThreads>;
  emptyMessage: string;
  ownerOf: SessionListViewModel["ownerOf"] | undefined;
  onSelectSession: (session: SessionListTarget) => void;
}) {
  const selection = use(SessionListGroupSelectionContext)?.selection;
  if (threads.length === 0) return <GroupNote>{emptyMessage}</GroupNote>;
  return (
    <SessionListThreadList
      {...props}
      threads={threads}
      ownerOf={ownerOf}
      onSelectSession={onSelectSession}
      selection={selection}
      emptyMessage={emptyMessage}
    />
  );
}

/** A failed refresh over loaded rows, and the group's own previous/next paging. */
function GroupPager({ catalog }: { catalog: GroupCatalog }) {
  return (
    <>
      {catalog.failed && catalog.sessions.length > 0 ? (
        <GroupNote>
          Couldn’t refresh sessions{" "}
          <Button variant="ghost" size="sm" onClick={catalog.retry}>
            Retry
          </Button>
        </GroupNote>
      ) : null}
      {catalog.paging || catalog.previous || catalog.next ? (
        <GroupNote>
          {catalog.paging ? <span role="status">Loading sessions…</span> : null}
          {catalog.previous ? (
            <Button
              variant="ghost"
              size="sm"
              disabled={catalog.paging}
              onClick={catalog.previousPage}
            >
              Previous sessions
            </Button>
          ) : null}
          {catalog.next ? (
            <Button variant="ghost" size="sm" disabled={catalog.paging} onClick={catalog.nextPage}>
              Next sessions
            </Button>
          ) : null}
        </GroupNote>
      ) : null}
    </>
  );
}

function selectionDragForPair(
  drag: SessionSelectionDrag | undefined,
  pair: SessionListPair | null
) {
  if (!drag) return undefined;
  return drag.pairKey === null || drag.pairKey === sessionListPairKey(pair) ? drag : undefined;
}
