import type { SessionThreadBranchChange } from "../../lib/session-thread-branch";
import { useState } from "react";

import { TriangleAlert, WifiOff } from "lucide-react";

import { Alert, AlertActions, AlertDescription, Button, SearchInput } from "@compozy/ui";

import { useSubagentContextSessions } from "../../hooks/use-subagent-context-sessions";
import { buildSessionListThreads, countThreadSessions } from "../../lib/session-list-threads";
import { SessionListGroupSelectionContext } from "../../contexts/session-list-group-selection-context-value";
import type { SessionListViewModel } from "../../hooks/use-session-list-view";
import type { SessionCreateOpenOptions } from "../../lib/session-create-target";
import type { SessionListTarget, SessionPayload } from "../../types";
import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import { emptyForScope } from "@/systems/profiles";
import { SessionListBadgeContext } from "../../lib/session-list-row-badge";
import { sessionListShelfNoun } from "../../lib/session-list-shelf";

import type { useSessionListBulkSelection } from "../../hooks/use-session-list-bulk-selection";
import { useSessionListSelection } from "../../hooks/use-session-list-selection";
import { useSessionListFolderLayout } from "../../hooks/use-session-list-folder-layout";
import type { SessionFolderDialogState } from "../../hooks/use-session-list-folder-layout";
import type { WorkspaceFolderPayload } from "../../types";
import { SessionFolderDialogs } from "./session-folder-dialogs";
import { SessionListTotals } from "./session-list-totals";
import { SessionListGroups } from "./session-list-groups";
import { SessionListSections } from "./session-list-sections";
import { SessionListSelectionBar } from "./session-list-selection-bar";
import { SessionListSelectionFolders } from "./session-list-selection-folders";
import { SessionListSkeleton } from "./session-list-skeleton";
import { SessionListThreadList } from "./session-list-thread-list";
import { SessionListToolbar } from "./session-list-toolbar";

export interface SessionListProps {
  sessions: readonly SessionPayload[];
  disconnected: boolean;
  collapsedThreadIds: readonly string[];
  subagentThreadIds: readonly string[];
  currentSessionId?: string;
  /** The session on screen; a subagent session is revealed under its parent row. */
  revealedSession?: SessionPayload | null;
  /** Breadth, order, grouping, and the flat catalog or the Group-by groups. */
  view: SessionListViewModel;
  onThreadBranchChange: SessionThreadBranchChange;
  onSelectSession: (session: SessionListTarget) => void;
  /** Opens session creation; `folderId` files the new session into that folder. */
  onNewSession: (options?: SessionCreateOpenOptions) => void;
  sessionActions: SessionLifecycleActionHandlers;
  testIdPrefix: string;
  /** Rendered above the controls (the modal's eyebrow + close chrome). */
  header?: (visibleCount: number | null) => React.ReactNode;
}

/**
 * Say what is empty AND for whom: an operator in Marketing needs to know the
 * list is empty in Marketing, not on the machine (US-009.EC-3).
 */
function emptySessionListMessage(filtering: boolean, view: SessionListViewModel): string {
  if (filtering) return `${emptyForScope("matching sessions", view.scopeLabel)}.`;
  return `${emptyForScope(sessionListShelfNoun(view.shelf), view.scopeLabel)}.`;
}

/**
 * Shared sessions catalog body for the topbar breadth: one flat list of
 * provenance threads by default — under All projects every row names its
 * project — or Group-by groups that page independently (ADR-004). Ordering is
 * served by the daemon for the operator's chosen sort — this component renders
 * the order it receives rather than re-deciding it.
 *
 * Loaded parents retain their descendants as threads. A parent outside the
 * current page leaves its child visible as a root rather than hiding it.
 */
export function SessionList({
  sessions,
  disconnected,
  collapsedThreadIds,
  subagentThreadIds,
  currentSessionId,
  revealedSession,
  view,
  onThreadBranchChange,
  onSelectSession,
  onNewSession,
  sessionActions,
  testIdPrefix,
  header,
}: SessionListProps) {
  const [filter, setFilter] = useState("");
  // A project folder dialog belongs to the lens it opened under: a profile or
  // scope change drops it, so it never submits a folder from the old profile.
  const [projectFolderDialogState, setProjectFolderDialogState] = useState<{
    lens: string;
    dialog: SessionFolderDialogState<WorkspaceFolderPayload>;
  }>({ lens: view.selectionKey, dialog: null });
  const projectFolderDialog =
    projectFolderDialogState.lens === view.selectionKey ? projectFolderDialogState.dialog : null;
  const setProjectFolderDialog = (dialog: SessionFolderDialogState<WorkspaceFolderPayload>) =>
    setProjectFolderDialogState({ lens: view.selectionKey, dialog });
  const projectLayout = view.projectFolders?.layout;
  const remoteSearch = view.search !== undefined;
  const normalizedFilter = (view.search ?? filter).trim().toLocaleLowerCase();
  const listed = useSubagentContextSessions(
    sessions,
    revealedSession,
    remoteSearch && normalizedFilter !== ""
  );
  const threads = buildSessionListThreads(listed, remoteSearch ? "" : normalizedFilter);
  const grouped = view.groupBy !== "none";
  // Grouped lists read each group separately; the flat list is empty, so the groups own the count.
  const visibleCount = sessionListVisibleCount(view, threads, normalizedFilter);
  const collapsedThreads = new Set(collapsedThreadIds);
  const subagentThreads = new Set(subagentThreadIds);
  const filtering = normalizedFilter !== "";
  const folders = useSessionListFolderLayout(view, threads, filtering);
  const { layout, sections } = folders;
  // A grouped list's groups each read their own page and publish their rows to it.
  const { bulk, groupSelection } = useSessionListSelection({
    selectionKey: view.selectionKey,
    shelf: view.shelf,
    grouped,
    sessions: listed,
    visibleThreads: folders.visibleThreads,
    collapsedThreads,
    layout,
    sessionActions,
  });

  return (
    <div
      className="flex min-h-0 flex-1 flex-col"
      data-testid={`${testIdPrefix}-content`}
      onKeyDown={bulk.onKeyDown}
    >
      {header?.(visibleCount)}
      <SessionListControls
        bulk={bulk}
        view={view}
        onNewSession={() => onNewSession()}
        {...listNewFolder(view, {
          session: layout ? () => folders.setDialog({ kind: "create" }) : undefined,
          project: projectLayout ? () => setProjectFolderDialog({ kind: "create" }) : undefined,
        })}
        testIdPrefix={testIdPrefix}
      />
      <div className="px-3 pb-1.5">
        <SearchInput
          value={view.search ?? filter}
          onChange={view.setSearch ?? setFilter}
          placeholder="Filter sessions…"
          aria-label="Filter sessions"
          containerClassName="min-w-0"
        />
      </div>
      <SessionListStatus disconnected={disconnected} view={view} />
      {grouped ? (
        <div
          className="flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto px-2.5 pt-0.5"
          data-group-by={view.groupBy}
        >
          <SessionListGroupSelectionContext value={groupSelection}>
            <SessionListGroups
              view={view}
              filtering={filtering}
              collapsedThreads={collapsedThreads}
              subagentThreads={subagentThreads}
              currentSessionId={currentSessionId}
              onThreadBranchChange={onThreadBranchChange}
              onSelectSession={onSelectSession}
              onNewSession={onNewSession}
              sessionActions={sessionActions}
              testIdPrefix={testIdPrefix}
              onProjectFolderDialog={setProjectFolderDialog}
            />
          </SessionListGroupSelectionContext>
        </div>
      ) : sections && layout ? (
        <SessionListFolderBody
          sections={sections}
          layout={layout}
          folders={folders}
          view={view}
          bulk={bulk}
          filtering={filtering}
          collapsedThreads={collapsedThreads}
          subagentThreads={subagentThreads}
          currentSessionId={currentSessionId}
          onThreadBranchChange={onThreadBranchChange}
          onSelectSession={onSelectSession}
          onNewSession={onNewSession}
          sessionActions={sessionActions}
          testIdPrefix={testIdPrefix}
        />
      ) : (
        <SessionListBody
          view={view}
          threads={threads}
          collapsedThreads={collapsedThreads}
          subagentThreads={subagentThreads}
          currentSessionId={currentSessionId}
          normalizedFilter={normalizedFilter}
          onThreadBranchChange={onThreadBranchChange}
          onSelectSession={onSelectSession}
          selection={bulk.rowSelection}
          sessionActions={sessionActions}
          testIdPrefix={testIdPrefix}
        />
      )}
      {layout ? (
        <SessionFolderDialogs
          state={folders.dialog}
          onClose={() => folders.setDialog(null)}
          onCreate={layout.createFolder}
          onRename={layout.renameFolder}
          onDelete={layout.deleteFolder}
        />
      ) : null}
      {projectLayout ? (
        <SessionFolderDialogs
          holds="projects"
          state={projectFolderDialog}
          onClose={() => setProjectFolderDialog(null)}
          onCreate={projectLayout.createFolder}
          onRename={projectLayout.renameFolder}
          onDelete={projectLayout.deleteFolder}
        />
      ) : null}
      <SessionListTotals totals={view.totals} testIdPrefix={testIdPrefix} />
      {bulk.folderMove ? (
        <SessionListSelectionFolders folders={bulk.folderMove} active={bulk.active} />
      ) : null}
    </div>
  );
}

/**
 * The toolbar's New folder: one pair's session folder, else the profile's project
 * folder under All projects by project, else disabled with the view's reason.
 */
function listNewFolder(
  view: SessionListViewModel,
  create: { session?: () => void; project?: () => void }
): { onNewFolder?: () => void; newFolderUnavailable: string | null } {
  const onNewFolder = create.session ?? create.project;
  return { onNewFolder, newFolderUnavailable: onNewFolder ? null : view.newFolderUnavailable };
}

function SessionListStatus({
  disconnected,
  view,
}: Pick<SessionListProps, "disconnected" | "view">) {
  return (
    <>
      {disconnected ? (
        <Alert className="mx-3 my-1 w-auto px-2.5 py-2" role="status" variant="warning">
          <WifiOff aria-hidden="true" />
          <AlertDescription>
            Can&apos;t get session updates right now. The sessions below may be out of date.
          </AlertDescription>
        </Alert>
      ) : null}
      {view.catalog?.failed && (view.catalog.sessions.length ?? 0) === 0 ? (
        <Alert className="mx-3 my-1 w-auto px-2.5 py-2" role="status" variant="warning">
          <TriangleAlert aria-hidden="true" />
          <AlertDescription>Couldn&rsquo;t load sessions.</AlertDescription>
          <AlertActions>
            <Button size="xs" type="button" variant="secondary" onClick={view.catalog.retry}>
              Retry
            </Button>
          </AlertActions>
        </Alert>
      ) : null}
      {view.layoutError ? (
        <Alert className="mx-3 my-1 w-auto px-2.5 py-2" role="status" variant="warning">
          <TriangleAlert aria-hidden="true" />
          <AlertDescription>Couldn&rsquo;t load folders.</AlertDescription>
          <AlertActions>
            <Button size="xs" type="button" variant="secondary" onClick={view.layoutError.retry}>
              Retry
            </Button>
          </AlertActions>
        </Alert>
      ) : null}
    </>
  );
}

type SessionListFolderLayoutModel = ReturnType<typeof useSessionListFolderLayout>;

/** The workspace list grouped into folders, with drag and folder verbs wired in. */
function SessionListFolderBody({
  sections,
  layout,
  folders,
  view,
  bulk,
  filtering,
  collapsedThreads,
  subagentThreads,
  currentSessionId,
  onThreadBranchChange,
  onSelectSession,
  onNewSession,
  sessionActions,
  testIdPrefix,
}: {
  sections: NonNullable<SessionListFolderLayoutModel["sections"]>;
  layout: NonNullable<SessionListFolderLayoutModel["layout"]>;
  folders: SessionListFolderLayoutModel;
  view: SessionListViewModel;
  bulk: ReturnType<typeof useSessionListBulkSelection>;
  filtering: boolean;
  collapsedThreads: ReadonlySet<string>;
  subagentThreads: ReadonlySet<string>;
  currentSessionId: SessionListProps["currentSessionId"];
  onThreadBranchChange: SessionListProps["onThreadBranchChange"];
  onSelectSession: SessionListProps["onSelectSession"];
  onNewSession: SessionListProps["onNewSession"];
  sessionActions: SessionListProps["sessionActions"];
  testIdPrefix: string;
}) {
  const { dnd } = folders;
  // Nothing has answered yet: placeholder rows, never the empty state.
  if (view.catalog?.loading) return <SessionListSkeleton />;
  return (
    <div
      className="flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto px-2.5 pt-0.5"
      aria-busy={view.catalog?.paging || undefined}
    >
      <SessionListSections
        sections={sections}
        layout={layout}
        dnd={dnd}
        selectionDrag={bulk.drag}
        orderable={folders.orderable}
        filtering={filtering}
        collapsedThreads={collapsedThreads}
        subagentThreads={subagentThreads}
        collapsedFolders={folders.collapsedFolders}
        currentSessionId={currentSessionId}
        onThreadBranchChange={onThreadBranchChange}
        onToggleFolder={folders.toggleFolder}
        onSelectSession={session => {
          if (!view.catalog?.paging) onSelectSession(session);
        }}
        onNewSessionInFolder={folderId => onNewSession({ folderId })}
        onFolderDialog={folders.setDialog}
        selection={bulk.rowSelection}
        sessionActions={sessionActions}
        testIdPrefix={testIdPrefix}
        emptyMessage={emptySessionListMessage(filtering, view)}
      />
      {view.catalog ? <SessionListCatalogNavigation catalog={view.catalog} /> : null}
    </div>
  );
}

function SessionListControls({
  bulk,
  view,
  onNewSession,
  onNewFolder,
  newFolderUnavailable,
  testIdPrefix,
}: {
  bulk: ReturnType<typeof useSessionListBulkSelection>;
  view: SessionListViewModel;
  onNewSession: () => void;
  onNewFolder?: () => void;
  newFolderUnavailable: string | null;
  testIdPrefix: string;
}) {
  return bulk.active ? (
    <SessionListSelectionBar {...bulk.bar} testIdPrefix={testIdPrefix} />
  ) : (
    <>
      <SessionListToolbar
        shelf={view.shelf}
        laterCount={view.laterCount}
        sort={view.sort}
        groupBy={view.groupBy}
        groupByOptions={view.groupByOptions}
        onShelfChange={view.setShelf}
        onSortChange={view.setSort}
        onGroupByChange={view.setGroupBy}
        onNewSession={onNewSession}
        onNewFolder={onNewFolder}
        newFolderUnavailable={newFolderUnavailable}
        testIdPrefix={testIdPrefix}
      />
      {view.sortNote ? (
        <p className="px-3 pb-1 text-micro text-subtle" data-testid={`${testIdPrefix}-sort-note`}>
          {view.sortNote}
        </p>
      ) : null}
    </>
  );
}

function SessionListBody({
  view,
  threads,
  collapsedThreads,
  subagentThreads,
  currentSessionId,
  normalizedFilter,
  onThreadBranchChange,
  onSelectSession,
  selection,
  sessionActions,
  testIdPrefix,
}: Pick<
  SessionListProps,
  | "view"
  | "currentSessionId"
  | "onThreadBranchChange"
  | "onSelectSession"
  | "sessionActions"
  | "testIdPrefix"
> & {
  threads: ReturnType<typeof buildSessionListThreads>;
  collapsedThreads: ReadonlySet<string>;
  subagentThreads: ReadonlySet<string>;
  normalizedFilter: string;
  selection: ReturnType<typeof useSessionListBulkSelection>["rowSelection"];
}) {
  const ownerOf = view.aggregate ? view.ownerOf : undefined;
  // Nothing has answered yet: placeholder rows, never the empty state.
  if (view.catalog?.loading) return <SessionListSkeleton />;
  return (
    <div
      className="flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto px-2.5 pt-0.5"
      aria-busy={view.catalog?.paging || undefined}
    >
      <SessionListBadgeContext value={view.badgeOf}>
        <SessionListThreadList
          threads={threads}
          collapsedThreads={collapsedThreads}
          subagentThreads={subagentThreads}
          currentSessionId={currentSessionId}
          ownerOf={ownerOf}
          onThreadBranchChange={onThreadBranchChange}
          onSelectSession={session => {
            if (!view.catalog?.paging) onSelectSession(session);
          }}
          selection={selection}
          sessionActions={sessionActions}
          testIdPrefix={testIdPrefix}
          emptyMessage={emptySessionListMessage(normalizedFilter !== "", view)}
        />
      </SessionListBadgeContext>
      {view.catalog ? <SessionListCatalogNavigation catalog={view.catalog} /> : null}
    </div>
  );
}

function SessionListCatalogNavigation({
  catalog,
}: {
  catalog: NonNullable<SessionListViewModel["catalog"]>;
}) {
  return (
    <div className="flex items-center justify-between px-2 py-2">
      {catalog.paging ? <span role="status">Loading sessions…</span> : null}
      {catalog.previous ? (
        <Button variant="ghost" size="sm" disabled={catalog.paging} onClick={catalog.previousPage}>
          Previous sessions
        </Button>
      ) : (
        <span />
      )}
      {catalog.next ? (
        <Button variant="ghost" size="sm" disabled={catalog.paging} onClick={catalog.nextPage}>
          Next sessions
        </Button>
      ) : null}
      {/* An empty failed list already states the failure with its one Retry above. */}
      {catalog.failed && catalog.sessions.length > 0 ? (
        <Button variant="ghost" size="sm" onClick={catalog.retry}>
          Retry loading sessions
        </Button>
      ) : null}
    </div>
  );
}

function sessionListVisibleCount(
  view: SessionListViewModel,
  threads: ReturnType<typeof buildSessionListThreads>,
  filter: string
): number | null {
  if (view.groupBy === "none") return countThreadSessions(threads);
  if (filter) return null;
  return view.groups.reduce((sum, group) => sum + group.total, 0);
}
