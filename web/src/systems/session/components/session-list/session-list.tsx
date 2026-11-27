import { useSelector } from "@xstate/store-react";
import type { ReactNode } from "react";

import { SearchInput } from "@compozy/ui";

import { useWorktreeScopeId } from "@/hooks/use-window-scope";
import { LOCAL_COMPUTER_ID } from "@/lib/computer-scope";
import { useComputers } from "@/systems/computers";

import { SessionMenuOpenContext } from "../../contexts/session-menu-open-context-value";
import type { ComputerSessionsSource } from "../../hooks/use-computer-sessions-sources";
import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import { useSessionListComputers } from "../../hooks/use-session-list-computers";
import { useSessionListFolderLayout } from "../../hooks/use-session-list-folder-layout";
import { useSessionListSearch } from "../../hooks/use-session-list-local-state";
import { useSessionListContent } from "../../hooks/use-session-list-reveal-current";
import { useSessionListSelection } from "../../hooks/use-session-list-selection";
import {
  resolveSessionWindowOrder,
  sessionSidebarStore,
} from "../../hooks/use-session-sidebar-state";
import type {
  SessionListTotals as Totals,
  SessionListViewModel,
} from "../../hooks/use-session-list-view";
import type { SessionCreateOpenOptions } from "../../lib/session-create-target";
import { buildSessionListThreads, countThreadSessions } from "../../lib/session-list-threads";
import { SESSION_LIST_NEW_FOLDER_MULTI_COMPUTER_NOTE } from "../../lib/session-list-view-helpers";
import { sessionThreadFolds } from "../../lib/session-sidebar-scope";
import type { SessionThreadBranchChange } from "../../lib/session-thread-branch";
import type { SessionListTarget, SessionPayload, WorkspaceFolderPayload } from "../../types";
import type { SessionFolderDialogState } from "../../hooks/use-session-list-folder-layout";
import { AlwaysOnListTop } from "../always-on/always-on-list-top";
import { ArchivedCleanup } from "./archived-cleanup";
import { SessionFolderDialogs } from "./session-folder-dialogs";
import { SessionListMultiComputerBody, SessionListSingleBody } from "./session-list-body";
import { SessionListControls } from "./session-list-controls";
import { SessionListPinned } from "./session-list-pinned";
import {
  SessionListOfflineNotice,
  SessionListScopeRemovedNotice,
} from "./session-list-scope-notice";
import { SessionListSelectionFolders } from "./session-list-selection-folders";
import { SessionListStatus } from "./session-list-status";
import { SessionListSubagentActionsHost } from "./session-list-subagent-actions";
import { SessionListTotals } from "./session-list-totals";

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
  header?: (visibleCount: number | null) => ReactNode;
  /** Optional notice rendered between the search bar and rows (`F8` offline/removed scope). */
  scopeNotice?: ReactNode;
  /** Opens computer details in Settings when an incompatible computer shows Details (`S6`). */
  onOpenComputerDetails?: (computerId: string) => void;
}

type MultiComputerListState = ReturnType<typeof useSessionListComputers>;

function aggregateSourcesTotals(
  sources: readonly ComputerSessionsSource[],
  shelf: SessionListViewModel["shelf"]
): Totals | undefined {
  if (shelf !== "active") return undefined;
  let answered = false;
  let needsYou = 0;
  let working = 0;
  let idle = 0;
  for (const source of sources) {
    if (source.read !== "ok" && source.read !== "offline") continue;
    const facets = source.facets?.facets;
    if (!facets) continue;
    answered = true;
    needsYou += facets.needs_you;
    working += facets.working;
    idle += facets.idle;
  }
  return answered ? { needsYou, working, idle } : undefined;
}

function aggregateLaterCount(sources: readonly ComputerSessionsSource[]): number | undefined {
  let answered = false;
  let total = 0;
  for (const source of sources) {
    if (source.read !== "ok" && source.read !== "offline") continue;
    if (source.facets === undefined) continue;
    answered = true;
    total += source.laterCount;
  }
  return answered ? total : undefined;
}

function resolveToolbarNewFolder(input: {
  multiComputerVisible: boolean;
  scopedRemoteSource: ComputerSessionsSource | null;
  view: SessionListViewModel;
  hasSessionLayout: boolean;
  hasProjectLayout: boolean;
  openSessionFolder: () => void;
  openProjectFolder: () => void;
}): { onNewFolder?: () => void; newFolderUnavailable: string | null } {
  const {
    multiComputerVisible,
    scopedRemoteSource,
    view,
    hasSessionLayout,
    hasProjectLayout,
    openSessionFolder,
    openProjectFolder,
  } = input;
  if (multiComputerVisible) {
    return {
      onNewFolder: undefined,
      newFolderUnavailable: SESSION_LIST_NEW_FOLDER_MULTI_COMPUTER_NOTE,
    };
  }
  if (scopedRemoteSource) {
    const operable = scopedRemoteSource.canOperate && scopedRemoteSource.read === "ok";
    if (operable && view.groupBy === "project" && view.allProjects) {
      return { onNewFolder: openProjectFolder, newFolderUnavailable: null };
    }
    return {
      onNewFolder: undefined,
      newFolderUnavailable: operable
        ? "Group by Project to add folders."
        : "Folders aren't available on this computer right now.",
    };
  }
  const onNewFolder = hasSessionLayout
    ? openSessionFolder
    : hasProjectLayout
      ? openProjectFolder
      : undefined;
  return { onNewFolder, newFolderUnavailable: onNewFolder ? null : view.newFolderUnavailable };
}

function resolveScopeNotice(multi: MultiComputerListState | null, testIdPrefix: string): ReactNode {
  if (!multi) return null;
  if (multi.computerScopeState.removedName) {
    return (
      <SessionListScopeRemovedNotice
        name={multi.computerScopeState.removedName}
        onDismiss={multi.computerScopeState.dismissRemovedNotice}
        testIdPrefix={testIdPrefix}
      />
    );
  }
  if (multi.scopedOfflineComputer) {
    const offlineComputer = multi.scopedOfflineComputer;
    return (
      <SessionListOfflineNotice
        computer={offlineComputer}
        onReconnect={() => multi.reconnectScopedComputer(offlineComputer.id)}
        reconnecting={multi.reconnectingScoped}
        testIdPrefix={testIdPrefix}
      />
    );
  }
  return null;
}

/**
 * Shared sessions catalog body for the topbar breadth: one flat list of
 * provenance threads, Group-by groups, or multi-computer sections (`F6`–`F9`).
 * When no remote computers are enabled, avoids mounting multi-computer hooks so
 * local-only windows issue zero extra workspace/profile/computer reads (`N6`).
 */
export function SessionList(props: SessionListProps) {
  const { canRead, enabledComputers } = useComputers({ passive: true });
  const scopeId = useWorktreeScopeId();
  const windowComputerScope = useSelector(
    sessionSidebarStore,
    snapshot => resolveSessionWindowOrder(snapshot.context.byScope, scopeId).computerScope
  );
  const needsComputers = (canRead && enabledComputers.length > 0) || windowComputerScope !== "all";
  if (!needsComputers) return <SessionListCore {...props} multi={null} />;
  return <SessionListWithComputers {...props} />;
}

function SessionListWithComputers(props: SessionListProps) {
  const multi = useSessionListComputers(props.view);
  return <SessionListCore {...props} multi={multi} />;
}

function SessionListCore({
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
  scopeNotice,
  onOpenComputerDetails,
  multi,
}: SessionListProps & { multi: MultiComputerListState | null }) {
  const { contentRef, sessionActions, announcement } = useSessionListContent(
    hostActions,
    testIdPrefix,
    currentSessionId
  );
  const {
    activeSearch,
    listed,
    normalizedFilter,
    projectFolderDialog,
    setFilter,
    setProjectFolderDialog,
    threads,
  } = useSessionListSearch(view, sessions, revealedSession);
  const projectLayout = view.projectFolders?.layout;
  const grouped = view.groupBy !== "none";
  const visibleCount = sessionListVisibleCount(view, threads, normalizedFilter);
  const filtering = normalizedFilter !== "";
  const { collapsedThreads, subagentThreads } = sessionThreadFolds({
    collapsedThreadIds,
    openThreadIds: view.openThreadIds,
    subagentThreadIds,
    foldAll: view.foldAll,
    filtering,
  });

  const multiComputerVisible = multi?.multiComputerVisible === true;
  const scopedRemoteSource = multi?.scopedRemoteSource ?? null;
  const isScopedToLocal = multi?.isScopedToLocal === true;
  const computersLayout = multi?.computersLayout ?? "separate";

  const pinnedSection = (
    <SessionListPinned
      view={view}
      pinnedOverride={multi?.multiComputerPinned}
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
  const { layout } = folders;
  const { bulk, groupSelection } = useSessionListSelection({
    selectionKey: view.selectionKey,
    shelf: view.shelf,
    grouped: grouped || multiComputerVisible || scopedRemoteSource !== null,
    sessions: listed,
    visibleThreads: folders.visibleThreads,
    collapsedThreads,
    layout,
    sessionActions,
  });

  const toolbarComputer =
    scopedRemoteSource?.key ?? (isScopedToLocal ? LOCAL_COMPUTER_ID : undefined);
  const toolbarNewSession = () => openToolbarSession(onNewSession, toolbarComputer);

  const toolbarNewFolder = resolveToolbarNewFolder({
    multiComputerVisible,
    scopedRemoteSource,
    view,
    hasSessionLayout: layout !== undefined,
    hasProjectLayout: projectLayout !== undefined,
    openSessionFolder: () => folders.setDialog({ kind: "create" }),
    openProjectFolder: () => setProjectFolderDialog({ kind: "create" }),
  });

  const computerLayoutOption =
    multiComputerVisible && view.setComputers
      ? { value: computersLayout, onValueChange: view.setComputers }
      : undefined;

  const { effectiveTotals, effectiveLaterCount } = listComputerCounts(
    view,
    multi,
    scopedRemoteSource
  );

  const showAlwaysOnTop =
    !scopedRemoteSource && (!multiComputerVisible || computersLayout === "together");
  const isRemoteOrMulti = multiComputerVisible || scopedRemoteSource !== null;

  return (
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
            archiveStoppedDisabled={multiComputerVisible}
            archiveStoppedSource={scopedRemoteSource}
            bulk={bulk}
            computerLayout={computerLayoutOption}
            disableAlwaysOnCreate={isRemoteOrMulti}
            laterCount={effectiveLaterCount}
            view={view}
            onNewSession={toolbarNewSession}
            {...toolbarNewFolder}
            testIdPrefix={testIdPrefix}
          />
          <div className="px-3 pb-1.5">
            <SearchInput
              value={activeSearch}
              onChange={view.setSearch ?? setFilter}
              placeholder="Filter sessions…"
              aria-label="Filter sessions"
              containerClassName="min-w-0"
            />
          </div>
          {scopeNotice ?? resolveScopeNotice(multi, testIdPrefix)}
          {showAlwaysOnTop ? (
            <AlwaysOnListTop
              currentSessionId={currentSessionId}
              groupBy={view.groupBy}
              onOpenSession={onSelectSession}
              workspaceId={view.focusedWorkspaceId}
            />
          ) : null}
          <SessionListStatus disconnected={disconnected} view={view} />
          {isRemoteOrMulti && multi ? (
            <SessionListMultiComputerBody
              bulk={bulk}
              collapsedThreads={collapsedThreads}
              computersLayout={computersLayout}
              currentSessionId={currentSessionId}
              filtering={filtering}
              groupSelection={groupSelection}
              localLens={multi.localLens}
              onNewSession={onNewSession}
              onOpenComputerDetails={onOpenComputerDetails}
              onProjectFolderDialog={setProjectFolderDialog}
              onSelectSession={onSelectSession}
              onThreadBranchChange={onThreadBranchChange}
              pinnedSection={pinnedSection}
              projectFolderDialog={scopedRemoteSource ? projectFolderDialog : undefined}
              scope={multi.dataScope}
              scopedSource={scopedRemoteSource}
              search={activeSearch}
              sessionActions={sessionActions}
              sources={multi.computerSources.sources}
              subagentThreads={subagentThreads}
              testIdPrefix={testIdPrefix}
              view={view}
            />
          ) : (
            <SessionListSingleBody
              bulk={bulk}
              collapsedThreads={collapsedThreads}
              currentSessionId={currentSessionId}
              filtering={filtering}
              folders={folders}
              groupSelection={groupSelection}
              normalizedFilter={normalizedFilter}
              onNewSession={onNewSession}
              onProjectFolderDialog={setProjectFolderDialog}
              onSelectSession={onSelectSession}
              onThreadBranchChange={onThreadBranchChange}
              pinnedSection={pinnedSection}
              sessionActions={sessionActions}
              subagentThreads={subagentThreads}
              testIdPrefix={testIdPrefix}
              threads={threads}
              view={view}
            />
          )}
          <SessionListLocalDialogs
            folders={folders}
            projectFolderDialog={projectFolderDialog}
            projectLayout={projectLayout}
            scopedRemoteSource={scopedRemoteSource}
            setProjectFolderDialog={setProjectFolderDialog}
          />
          <SessionListTotals
            totals={effectiveTotals}
            uncountedNames={
              multiComputerVisible ? multi!.computerSources.uncountedNames : undefined
            }
            testIdPrefix={testIdPrefix}
          />
          {!isRemoteOrMulti ? <ArchivedCleanup view={view} testIdPrefix={testIdPrefix} /> : null}
          {bulk.folderMove ? (
            <SessionListSelectionFolders folders={bulk.folderMove} active={bulk.active} />
          ) : null}
        </div>
      </SessionListSubagentActionsHost>
    </SessionMenuOpenContext>
  );
}

function openToolbarSession(
  open: SessionListProps["onNewSession"],
  computerId: ComputerSessionsSource["key"] | undefined
) {
  if (computerId) open({ computerId });
  else open();
}

function listComputerCounts(
  view: SessionListViewModel,
  multi: MultiComputerListState | null,
  scoped: ComputerSessionsSource | null
) {
  if (multi?.multiComputerVisible) {
    const { sources } = multi.computerSources;
    return {
      effectiveTotals: aggregateSourcesTotals(sources, view.shelf) ?? view.totals,
      effectiveLaterCount: aggregateLaterCount(sources) ?? view.laterCount,
    };
  }
  return {
    effectiveTotals: scoped ? aggregateSourcesTotals([scoped], view.shelf) : view.totals,
    effectiveLaterCount: scoped?.laterCount ?? view.laterCount,
  };
}

function SessionListLocalDialogs({
  folders,
  projectLayout,
  projectFolderDialog,
  setProjectFolderDialog,
  scopedRemoteSource,
}: {
  folders: ReturnType<typeof useSessionListFolderLayout>;
  projectLayout: NonNullable<SessionListViewModel["projectFolders"]>["layout"] | undefined;
  projectFolderDialog: SessionFolderDialogState<WorkspaceFolderPayload>;
  setProjectFolderDialog: (state: SessionFolderDialogState<WorkspaceFolderPayload>) => void;
  scopedRemoteSource: ComputerSessionsSource | null;
}) {
  if (scopedRemoteSource) return null;
  return (
    <>
      {folders.layout ? (
        <SessionFolderDialogs
          state={folders.dialog}
          onClose={() => folders.setDialog(null)}
          onCreate={folders.layout.createFolder}
          onRename={folders.layout.renameFolder}
          onDelete={folders.layout.deleteFolder}
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
    </>
  );
}

function sessionListVisibleCount(
  view: SessionListViewModel,
  threads: ReturnType<typeof buildSessionListThreads>,
  filter: string
): number | null {
  const pinned = view.pinned?.sessions.length ?? 0;
  if (view.groupBy === "none") return countThreadSessions(threads) + pinned;
  if (filter) return null;
  return view.groups.reduce((sum, group) => sum + group.total, pinned);
}
