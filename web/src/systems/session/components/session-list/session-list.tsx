import type { SessionThreadBranchChange } from "../../lib/session-thread-branch";

import { SearchInput } from "@compozy/ui";

import { useSubagentContextSessions } from "../../hooks/use-subagent-context-sessions";
import { buildSessionListThreads, countThreadSessions } from "../../lib/session-list-threads";
import { SessionMenuOpenContext } from "../../contexts/session-menu-open-context-value";
import { SessionListGroupSelectionContext } from "../../contexts/session-list-group-selection-context-value";
import type { SessionListViewModel } from "../../hooks/use-session-list-view";
import type { SessionCreateOpenOptions } from "../../lib/session-create-target";
import type { SessionListTarget, SessionPayload } from "../../types";
import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import { emptyForScope } from "@/systems/profiles";
import { SessionListBadgeContext } from "../../lib/session-list-row-badge";
import { sessionListShelfNoun } from "../../lib/session-list-shelf";
import { sessionListHasPinnedRows } from "../../lib/session-list-pinned";
import { useSessionListPin } from "../../hooks/use-session-pin-actions";
import { useSessionListLocalState } from "../../hooks/use-session-list-local-state";
import { useSessionListRevealCurrent } from "../../hooks/use-session-list-reveal-current";
import { SessionListCatalogNavigation, SessionListControls } from "./session-list-controls";
import { SessionListPinned } from "./session-list-pinned";

import type { useSessionListBulkSelection } from "../../hooks/use-session-list-bulk-selection";
import { useSessionListSelection } from "../../hooks/use-session-list-selection";
import { useSessionListFolderLayout } from "../../hooks/use-session-list-folder-layout";
import { AlwaysOnListTop } from "../always-on/always-on-list-top";
import { ArchivedCleanup } from "./archived-cleanup";
import { SessionFolderDialogs } from "./session-folder-dialogs";
import { SessionListTotals } from "./session-list-totals";
import { SessionListGroups } from "./session-list-groups";
import { SessionListStatus } from "./session-list-status";
import { SessionListSections } from "./session-list-sections";
import { SessionListSelectionFolders } from "./session-list-selection-folders";
import { SessionListSkeleton } from "./session-list-skeleton";
import { SessionListThreadList } from "./session-list-thread-list";
import { SessionListSubagentActionsHost } from "./session-list-subagent-actions";
import {
  sessionThreadFolds,
  type SessionFoldSet,
  type SessionSubagentFolds,
} from "../../lib/session-sidebar-scope";

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
function emptySessionListMessage(filtering: boolean, view: SessionListViewModel): string | null {
  // Every session the list holds sits in Pinned above: the list is not empty.
  if (sessionListHasPinnedRows(view)) return null;
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
  sessionActions: hostActions,
  testIdPrefix,
  header,
}: SessionListProps) {
  // The list owns its Pinned section, so it owns Pin to top and Unpin: a board card has neither.
  const { contentRef, sessionActions, announcement } = useSessionListPin(hostActions, testIdPrefix);
  useSessionListRevealCurrent(contentRef, currentSessionId, sessions);
  const { filter, setFilter, projectFolderDialog, setProjectFolderDialog } =
    useSessionListLocalState(view.selectionKey);
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
  const filtering = normalizedFilter !== "";
  const { collapsedThreads, subagentThreads } = sessionThreadFolds({
    collapsedThreadIds,
    openThreadIds: view.openThreadIds,
    subagentThreadIds,
    foldAll: view.foldAll,
    filtering,
  });
  const pinnedSection = (
    <SessionListPinned
      view={view}
      filtering={filtering}
      collapsedThreads={collapsedThreads}
      subagentThreads={subagentThreads}
      currentSessionId={currentSessionId}
      onThreadBranchChange={onThreadBranchChange}
      onSelectSession={onSelectSession}
      sessionActions={sessionActions}
      testIdPrefix={testIdPrefix}
    />
  );
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
    // A row's menu opens a rolled-up item's child session the way the row itself opens.
    <SessionMenuOpenContext value={onSelectSession}>
      <SessionListSubagentActionsHost>
        <div
          className="flex min-h-0 flex-1 flex-col"
          data-testid={`${testIdPrefix}-content`}
          onKeyDown={bulk.onKeyDown}
          ref={contentRef}
        >
          <span role="status" aria-live="polite" className="sr-only">
            {announcement}
          </span>
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
          <AlwaysOnListTop
            currentSessionId={currentSessionId}
            groupBy={view.groupBy}
            onOpenSession={onSelectSession}
            workspaceId={view.focusedWorkspaceId}
          />
          <SessionListStatus disconnected={disconnected} view={view} />
          {grouped ? (
            <div
              className="flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto px-2.5 pt-0.5"
              data-group-by={view.groupBy}
            >
              {pinnedSection}
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
              pinnedSection={pinnedSection}
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
              pinnedSection={pinnedSection}
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
          <ArchivedCleanup view={view} testIdPrefix={testIdPrefix} />
          {bulk.folderMove ? (
            <SessionListSelectionFolders folders={bulk.folderMove} active={bulk.active} />
          ) : null}
        </div>
      </SessionListSubagentActionsHost>
    </SessionMenuOpenContext>
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
  pinnedSection,
}: {
  pinnedSection: React.ReactNode;
  sections: NonNullable<SessionListFolderLayoutModel["sections"]>;
  layout: NonNullable<SessionListFolderLayoutModel["layout"]>;
  folders: SessionListFolderLayoutModel;
  view: SessionListViewModel;
  bulk: ReturnType<typeof useSessionListBulkSelection>;
  filtering: boolean;
  collapsedThreads: SessionFoldSet;
  subagentThreads: SessionSubagentFolds;
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
      {pinnedSection}
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
  pinnedSection,
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
  collapsedThreads: SessionFoldSet;
  subagentThreads: SessionSubagentFolds;
  normalizedFilter: string;
  selection: ReturnType<typeof useSessionListBulkSelection>["rowSelection"];
  pinnedSection: React.ReactNode;
}) {
  const ownerOf = view.aggregate ? view.ownerOf : undefined;
  // Nothing has answered yet: placeholder rows, never the empty state.
  if (view.catalog?.loading) return <SessionListSkeleton />;
  return (
    <div
      className="flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto px-2.5 pt-0.5"
      aria-busy={view.catalog?.paging || undefined}
    >
      {pinnedSection}
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

function sessionListVisibleCount(
  view: SessionListViewModel,
  threads: ReturnType<typeof buildSessionListThreads>,
  filter: string
): number | null {
  // The list's own reads leave pinned sessions to the Pinned section, which counts them back.
  const pinned = view.pinned?.sessions.length ?? 0;
  if (view.groupBy === "none") return countThreadSessions(threads) + pinned;
  if (filter) return null;
  return view.groups.reduce((sum, group) => sum + group.total, pinned);
}
