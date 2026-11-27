import type { ReactNode } from "react";

import { cn } from "@compozy/ui";

import { emptyForScope, type ProfileScopeParams } from "@/systems/profiles";
import type { DataScope } from "@/systems/workspace";

import {
  SessionListGroupSelectionContext,
  type SessionListGroupSelection,
} from "../../contexts/session-list-group-selection-context-value";
import type { ComputerSessionsSource } from "../../hooks/use-computer-sessions-sources";
import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import type { useSessionListBulkSelection } from "../../hooks/use-session-list-bulk-selection";
import type { SessionSelectionDrag } from "../../hooks/use-session-list-dnd";
import type {
  SessionFolderDialogState,
  useSessionListFolderLayout,
} from "../../hooks/use-session-list-folder-layout";
import type { SessionRowSelection } from "../../hooks/use-session-selection";
import type { SessionListViewModel } from "../../hooks/use-session-list-view";
import type { SessionCreateOpenOptions } from "../../lib/session-create-target";
import { sessionListHasPinnedRows } from "../../lib/session-list-pinned";
import type { SessionListComputerLayout } from "../../lib/session-list-preferences";
import { SessionListBadgeContext } from "../../lib/session-list-row-badge";
import { sessionListShelfNoun } from "../../lib/session-list-shelf";
import type { buildSessionListThreads } from "../../lib/session-list-threads";
import type { SessionFoldSet, SessionSubagentFolds } from "../../lib/session-sidebar-scope";
import type { SessionThreadBranchChange } from "../../lib/session-thread-branch";
import type { SessionListTarget, WorkspaceFolderPayload } from "../../types";
import { SessionListComputerBody } from "./session-list-computer-body";
import {
  SessionListTogetherFlatBody,
  SessionListTogetherProfileGroups,
  SessionListTogetherProjectGroups,
} from "./session-list-computer-groups";
import { SessionListComputerSection } from "./session-list-computer-section";
import { SessionListCatalogNavigation } from "./session-list-controls";
import { SessionListGroups } from "./session-list-groups";
import { SessionListSections } from "./session-list-sections";
import { SessionListSkeleton } from "./session-list-skeleton";
import { SessionListThreadList } from "./session-list-thread-list";

type SessionListFolderLayoutModel = ReturnType<typeof useSessionListFolderLayout>;

/**
 * Say what is empty AND for whom: an operator in Marketing needs to know the
 * list is empty in Marketing, not on the machine (US-009.EC-3).
 */
function emptySessionListMessage(filtering: boolean, view: SessionListViewModel): string | null {
  if (sessionListHasPinnedRows(view)) return null;
  if (filtering) return `${emptyForScope("matching sessions", view.scopeLabel)}.`;
  return `${emptyForScope(sessionListShelfNoun(view.shelf), view.scopeLabel)}.`;
}

export interface SessionListSingleBodyProps {
  pinnedSection: ReactNode;
  view: SessionListViewModel;
  threads: ReturnType<typeof buildSessionListThreads>;
  folders: SessionListFolderLayoutModel;
  bulk?: ReturnType<typeof useSessionListBulkSelection>;
  selection?: SessionRowSelection;
  selectionDrag?: SessionSelectionDrag;
  groupSelection?: SessionListGroupSelection;
  rankPrefix?: readonly number[];
  filtering: boolean;
  normalizedFilter: string;
  collapsedThreads: SessionFoldSet;
  subagentThreads: SessionSubagentFolds;
  currentSessionId?: string;
  onThreadBranchChange: SessionThreadBranchChange;
  onSelectSession: (session: SessionListTarget) => void;
  onNewSession: (options?: SessionCreateOpenOptions) => void;
  onProjectFolderDialog: (state: SessionFolderDialogState<WorkspaceFolderPayload>) => void;
  sessionActions: SessionLifecycleActionHandlers;
  testIdPrefix: string;
  /** When true, omits the outer scroll container (used inside a computer section). */
  embedded?: boolean;
  readOnly?: boolean;
}

function singleBodyScrollClass(embedded: boolean): string {
  return embedded
    ? "flex flex-col gap-0.5"
    : "flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto px-2.5 pt-0.5";
}

function SessionListGroupedBody(props: SessionListSingleBodyProps) {
  const { view, groupSelection, pinnedSection, embedded = false } = props;
  const groupTree = (
    <SessionListGroups
      view={view}
      filtering={props.filtering}
      collapsedThreads={props.collapsedThreads}
      subagentThreads={props.subagentThreads}
      currentSessionId={props.currentSessionId}
      onThreadBranchChange={props.onThreadBranchChange}
      onSelectSession={props.onSelectSession}
      onNewSession={props.onNewSession}
      sessionActions={props.sessionActions}
      testIdPrefix={props.testIdPrefix}
      onProjectFolderDialog={props.onProjectFolderDialog}
      rankPrefix={props.rankPrefix}
    />
  );
  return (
    <div
      className={singleBodyScrollClass(embedded)}
      data-session-list-scroll={embedded ? undefined : true}
      data-group-by={view.groupBy}
    >
      {pinnedSection}
      {groupSelection ? (
        <SessionListGroupSelectionContext value={groupSelection}>
          {groupTree}
        </SessionListGroupSelectionContext>
      ) : (
        groupTree
      )}
    </div>
  );
}

function SessionListSectionsBody(props: SessionListSingleBodyProps) {
  const { view, folders, pinnedSection, embedded = false } = props;
  const { layout, sections } = folders;
  if (!sections || !layout) return null;
  if (view.catalog?.loading && !view.partitions) return <SessionListSkeleton />;
  const rowSelection = props.selection ?? props.bulk?.rowSelection;
  const rowDrag = props.selectionDrag ?? props.bulk?.drag;
  return (
    <div
      className={singleBodyScrollClass(embedded)}
      data-session-list-scroll={embedded ? undefined : true}
      aria-busy={view.catalog?.paging || undefined}
    >
      {pinnedSection}
      <SessionListSections
        sections={sections}
        partitions={view.partitions}
        layout={layout}
        dnd={folders.dnd}
        selectionDrag={rowDrag}
        orderable={folders.orderable}
        filtering={props.filtering}
        collapsedThreads={props.collapsedThreads}
        subagentThreads={props.subagentThreads}
        collapsedFolders={folders.collapsedFolders}
        currentSessionId={props.currentSessionId}
        onThreadBranchChange={props.onThreadBranchChange}
        onToggleFolder={folders.toggleFolder}
        onSelectSession={session => {
          if (view.partitions || !view.catalog?.paging) props.onSelectSession(session);
        }}
        onNewSessionInFolder={folderId => props.onNewSession({ folderId })}
        onFolderDialog={folders.setDialog}
        selection={rowSelection}
        sessionActions={props.sessionActions}
        testIdPrefix={props.testIdPrefix}
        emptyMessage={emptySessionListMessage(props.filtering, view)}
      />
      {view.catalog && !view.partitions ? (
        <SessionListCatalogNavigation catalog={view.catalog} />
      ) : null}
    </div>
  );
}

function SessionListFlatBody(props: SessionListSingleBodyProps) {
  const { view, pinnedSection, embedded = false, readOnly = false } = props;
  if (view.catalog?.loading) return <SessionListSkeleton />;
  const ownerOf = view.aggregate ? view.ownerOf : undefined;
  const rowSelection = props.selection ?? props.bulk?.rowSelection;
  return (
    <div
      className={singleBodyScrollClass(embedded)}
      data-session-list-scroll={embedded ? undefined : true}
      aria-busy={view.catalog?.paging || undefined}
    >
      {pinnedSection}
      <SessionListBadgeContext value={view.badgeOf}>
        <SessionListThreadList
          threads={props.threads}
          collapsedThreads={props.collapsedThreads}
          subagentThreads={props.subagentThreads}
          currentSessionId={props.currentSessionId}
          ownerOf={ownerOf}
          onThreadBranchChange={props.onThreadBranchChange}
          onSelectSession={session => {
            if (!view.catalog?.paging) props.onSelectSession(session);
          }}
          selection={readOnly ? undefined : rowSelection}
          readOnly={readOnly}
          sessionActions={props.sessionActions}
          testIdPrefix={props.testIdPrefix}
          emptyMessage={emptySessionListMessage(props.normalizedFilter !== "", view)}
        />
      </SessionListBadgeContext>
      {view.catalog ? <SessionListCatalogNavigation catalog={view.catalog} /> : null}
    </div>
  );
}

/** One daemon's list body: grouped, folder-sectioned, or flat. */
export function SessionListSingleBody(props: SessionListSingleBodyProps) {
  if (props.view.groupBy !== "none") return <SessionListGroupedBody {...props} />;
  if (props.folders.sections && props.folders.layout && !props.readOnly) {
    return <SessionListSectionsBody {...props} />;
  }
  return <SessionListFlatBody {...props} />;
}

export interface SessionListMultiComputerBodyProps {
  pinnedSection: ReactNode;
  sources: readonly ComputerSessionsSource[];
  /** When set, renders this single computer with no section header (`F8`). */
  scopedSource: ComputerSessionsSource | null;
  computersLayout: SessionListComputerLayout;
  scope: DataScope | null;
  localLens?: ProfileScopeParams;
  search: string;
  view: SessionListViewModel;
  bulk: ReturnType<typeof useSessionListBulkSelection>;
  groupSelection: SessionListGroupSelection;
  filtering: boolean;
  collapsedThreads: SessionFoldSet;
  subagentThreads: SessionSubagentFolds;
  currentSessionId?: string;
  onThreadBranchChange: SessionThreadBranchChange;
  onSelectSession: (session: SessionListTarget) => void;
  onNewSession: (options?: SessionCreateOpenOptions) => void;
  projectFolderDialog?: SessionFolderDialogState<WorkspaceFolderPayload>;
  onProjectFolderDialog: (state: SessionFolderDialogState<WorkspaceFolderPayload>) => void;
  sessionActions: SessionLifecycleActionHandlers;
  testIdPrefix: string;
  onOpenComputerDetails?: (computerId: string) => void;
}

/**
 * Multi-computer body switching cleanly across Scoped (`F8`), Separate (`F7`),
 * and Together (`F6`) without duplicating list primitives.
 */
export function SessionListMultiComputerBody({
  pinnedSection,
  sources,
  scopedSource,
  computersLayout,
  scope,
  localLens,
  search,
  view,
  bulk,
  groupSelection,
  filtering,
  collapsedThreads,
  subagentThreads,
  currentSessionId,
  onThreadBranchChange,
  onSelectSession,
  onNewSession,
  projectFolderDialog,
  onProjectFolderDialog,
  sessionActions,
  testIdPrefix,
  onOpenComputerDetails,
}: SessionListMultiComputerBodyProps) {
  const separate = scopedSource === null && computersLayout === "separate";
  const emptyMessage = emptySessionListMessage(filtering, view);
  return (
    <div
      className={cn(
        "flex min-h-0 flex-1 flex-col overflow-y-auto px-2.5 pt-0.5 pb-2",
        separate
          ? "gap-2 [&>section+section]:border-t [&>section+section]:border-line-soft [&>section+section]:pt-2"
          : "gap-1"
      )}
      data-computer-layout={scopedSource ? "scoped" : computersLayout}
      data-group-by={view.groupBy}
      data-session-list-scroll
    >
      {pinnedSection}
      <SessionListGroupSelectionContext value={groupSelection}>
        {scopedSource ? (
          <SessionListComputerBody
            collapsedThreads={collapsedThreads}
            currentSessionId={currentSessionId}
            onNewSession={onNewSession}
            onOpenComputerDetails={onOpenComputerDetails}
            onProjectFolderDialogChange={onProjectFolderDialog}
            onSelectSession={onSelectSession}
            onThreadBranchChange={onThreadBranchChange}
            projectFolderDialog={projectFolderDialog}
            scoped
            search={search}
            sectionRank={0}
            sessionActions={sessionActions}
            source={scopedSource}
            subagentThreads={subagentThreads}
            testIdPrefix={testIdPrefix}
            worktreeId={view.worktreeId}
          />
        ) : computersLayout === "separate" ? (
          sources.map((source, index) => (
            <SessionListComputerSection
              collapsedThreads={collapsedThreads}
              currentSessionId={currentSessionId}
              groupBy={view.groupBy}
              key={source.key}
              localLens={localLens}
              onNewSession={onNewSession}
              onOpenComputerDetails={onOpenComputerDetails}
              onSelectSession={onSelectSession}
              onThreadBranchChange={onThreadBranchChange}
              scope={scope}
              search={search}
              sectionRank={index}
              sessionActions={sessionActions}
              shelf={view.shelf}
              showAutomation={view.showAutomation}
              source={source}
              subagentThreads={subagentThreads}
              testIdPrefix={testIdPrefix}
              worktreeId={view.worktreeId}
            />
          ))
        ) : view.groupBy === "project" ? (
          <SessionListTogetherProjectGroups
            collapsedThreads={collapsedThreads}
            currentSessionId={currentSessionId}
            emptyMessage={emptyMessage}
            filtering={filtering}
            onNewSession={onNewSession}
            onSelectSession={onSelectSession}
            onThreadBranchChange={onThreadBranchChange}
            sessionActions={sessionActions}
            sources={sources}
            subagentThreads={subagentThreads}
            testIdPrefix={testIdPrefix}
            view={view}
          />
        ) : view.groupBy === "profile" ? (
          <SessionListTogetherProfileGroups
            collapsedThreads={collapsedThreads}
            currentSessionId={currentSessionId}
            emptyMessage={emptyMessage}
            filtering={filtering}
            onNewSession={onNewSession}
            onSelectSession={onSelectSession}
            onThreadBranchChange={onThreadBranchChange}
            scope={scope}
            sessionActions={sessionActions}
            sources={sources}
            subagentThreads={subagentThreads}
            testIdPrefix={testIdPrefix}
            view={view}
          />
        ) : (
          <SessionListTogetherFlatBody
            collapsedThreads={collapsedThreads}
            currentSessionId={currentSessionId}
            emptyMessage={emptyMessage}
            filtering={filtering}
            onSelectSession={onSelectSession}
            onThreadBranchChange={onThreadBranchChange}
            scope={scope}
            selection={bulk.rowSelection}
            sessionActions={sessionActions}
            sources={sources}
            subagentThreads={subagentThreads}
            testIdPrefix={testIdPrefix}
            view={view}
          />
        )}
      </SessionListGroupSelectionContext>
    </div>
  );
}
