import { QueryClientContext } from "@tanstack/react-query";
import { useContext, useState, type ReactNode } from "react";

import { SearchInput, toast } from "@compozy/ui";

import { isRemoteComputerId, LOCAL_COMPUTER_ID } from "@/lib/computer-scope";
import { computerRegistryKeys, patchComputer } from "@/systems/computers";
import type { ComputerFixture } from "@/systems/computers/mocks/remote-computers-types";
import { useProfileReadScope } from "@/systems/profiles";
import { useActiveWorkspace } from "@/systems/workspace";

import { SessionMenuOpenContext } from "../../contexts/session-menu-open-context-value";
import {
  useComputerSessionsSources,
  type ComputerSessionsSource,
} from "../../hooks/use-computer-sessions-sources";
import { useSessionComputerScope } from "../../hooks/use-session-computer-scope";
import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import { useSessionListFolderLayout } from "../../hooks/use-session-list-folder-layout";
import { useSessionListLocalState } from "../../hooks/use-session-list-local-state";
import { useSessionListContent } from "../../hooks/use-session-list-reveal-current";
import { useSessionListSelection } from "../../hooks/use-session-list-selection";
import type { SessionListTotals as Totals, SessionListViewModel } from "../../hooks/use-session-list-view";
import { useSubagentContextSessions } from "../../hooks/use-subagent-context-sessions";
import type { SessionCreateOpenOptions } from "../../lib/session-create-target";
import { buildSessionListThreads, countThreadSessions } from "../../lib/session-list-threads";
import { SESSION_LIST_NEW_FOLDER_MULTI_COMPUTER_NOTE } from "../../lib/session-list-view-helpers";
import { sessionThreadFolds } from "../../lib/session-sidebar-scope";
import type { SessionThreadBranchChange } from "../../lib/session-thread-branch";
import type { SessionListTarget, SessionPayload } from "../../types";
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

/**
 * Shared sessions catalog body for the topbar breadth: one flat list of
 * provenance threads, Group-by groups, or multi-computer sections (`F6`–`F9`).
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
  scopeNotice,
  onOpenComputerDetails,
}: SessionListProps) {
  const queryClient = useContext(QueryClientContext);
  const [reconnectingScoped, setReconnectingScoped] = useState(false);
  const { dataScope } = useActiveWorkspace();
  const profile = useProfileReadScope();
  const { contentRef, sessionActions, announcement } = useSessionListContent(
    hostActions,
    testIdPrefix,
    currentSessionId
  );
  const { filter, setFilter, projectFolderDialog, setProjectFolderDialog } =
    useSessionListLocalState(view.selectionKey);
  const projectLayout = view.projectFolders?.layout;
  const remoteSearch = view.search !== undefined;
  const activeSearch = view.search ?? filter;
  const normalizedFilter = activeSearch.trim().toLocaleLowerCase();
  const listed = useSubagentContextSessions(
    sessions,
    revealedSession,
    remoteSearch && normalizedFilter !== ""
  );
  const threads = buildSessionListThreads(listed, remoteSearch ? "" : normalizedFilter);
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

  const computerSources = useComputerSessionsSources({
    scope: dataScope,
    lens: profile.params,
    shelf: view.shelf,
    showAutomation: view.showAutomation,
    search: view.groupFilters.q,
    worktreeId: view.worktreeId,
  });
  const computerScopeState = useSessionComputerScope(computerSources.remoteComputers);
  const hasRemote = computerSources.hasRemoteComputers;
  const effectiveComputerScope = hasRemote ? computerScopeState.effectiveScope : "all";
  const isScopedToLocal = hasRemote && effectiveComputerScope === LOCAL_COMPUTER_ID;
  const scopedRemoteSource =
    hasRemote && isRemoteComputerId(effectiveComputerScope)
      ? (computerSources.sources.find(source => source.key === effectiveComputerScope) ?? null)
      : null;
  const multiComputerVisible = hasRemote && effectiveComputerScope === "all";
  const computersLayout = view.computers ?? "separate";

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
  const { layout } = folders;
  const { bulk, groupSelection } = useSessionListSelection({
    selectionKey: view.selectionKey,
    shelf: view.shelf,
    grouped: grouped || multiComputerVisible,
    sessions: listed,
    visibleThreads: folders.visibleThreads,
    collapsedThreads,
    layout,
    sessionActions,
  });

  const toolbarNewSession = () => {
    if (scopedRemoteSource) {
      onNewSession({ computerId: scopedRemoteSource.key });
      return;
    }
    if (isScopedToLocal) {
      onNewSession({ computerId: LOCAL_COMPUTER_ID });
      return;
    }
    onNewSession();
  };

  const toolbarNewFolder = multiComputerVisible
    ? { onNewFolder: undefined, newFolderUnavailable: SESSION_LIST_NEW_FOLDER_MULTI_COMPUTER_NOTE }
    : listNewFolder(view, {
        session: layout ? () => folders.setDialog({ kind: "create" }) : undefined,
        project: projectLayout ? () => setProjectFolderDialog({ kind: "create" }) : undefined,
      });

  const computerLayoutOption =
    multiComputerVisible && view.setComputers
      ? { value: computersLayout, onValueChange: view.setComputers }
      : undefined;

  const effectiveTotals = multiComputerVisible
    ? (aggregateSourcesTotals(computerSources.sources, view.shelf) ?? view.totals)
    : scopedRemoteSource
      ? aggregateSourcesTotals([scopedRemoteSource], view.shelf)
      : view.totals;
  const effectiveLaterCount = multiComputerVisible
    ? (aggregateLaterCount(computerSources.sources) ?? view.laterCount)
    : scopedRemoteSource
      ? scopedRemoteSource.laterCount
      : view.laterCount;

  // Always-on top strip shows under Together or a local scope, not above Separate computer sections (`R1`).
  const showAlwaysOnTop =
    !scopedRemoteSource && (!multiComputerVisible || computersLayout === "together");

  const scopedOfflineComputer =
    scopedRemoteSource &&
    (scopedRemoteSource.read === "offline" || scopedRemoteSource.read === "offline_empty")
      ? (scopedRemoteSource.computer as ComputerFixture)
      : null;

  const builtInNotice = computerScopeState.removedName ? (
    <SessionListScopeRemovedNotice
      name={computerScopeState.removedName}
      onDismiss={computerScopeState.dismissRemoved}
      testIdPrefix={testIdPrefix}
    />
  ) : scopedOfflineComputer ? (
    <SessionListOfflineNotice
      computer={scopedOfflineComputer}
      onReconnect={() => {
        setReconnectingScoped(true);
        void patchComputer(scopedOfflineComputer.id, { enabled: true })
          .then(() => queryClient?.invalidateQueries({ queryKey: computerRegistryKeys.all }))
          .catch(err => {
            toast.error(err instanceof Error ? err.message : "Failed to reconnect computer.");
          })
          .finally(() => setReconnectingScoped(false));
      }}
      reconnecting={reconnectingScoped}
      testIdPrefix={testIdPrefix}
    />
  ) : null;

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
            bulk={bulk}
            computerLayout={computerLayoutOption}
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
          {scopeNotice ?? builtInNotice}
          {showAlwaysOnTop ? (
            <AlwaysOnListTop
              currentSessionId={currentSessionId}
              groupBy={view.groupBy}
              onOpenSession={onSelectSession}
              workspaceId={view.focusedWorkspaceId}
            />
          ) : null}
          <SessionListStatus disconnected={disconnected} view={view} />
          {multiComputerVisible || scopedRemoteSource ? (
            <SessionListMultiComputerBody
              bulk={bulk}
              collapsedThreads={collapsedThreads}
              computersLayout={computersLayout}
              currentSessionId={currentSessionId}
              filtering={filtering}
              groupSelection={groupSelection}
              onNewSession={onNewSession}
              onOpenComputerDetails={onOpenComputerDetails}
              onProjectFolderDialog={setProjectFolderDialog}
              onSelectSession={onSelectSession}
              onThreadBranchChange={onThreadBranchChange}
              pinnedSection={pinnedSection}
              scope={dataScope}
              scopedSource={scopedRemoteSource}
              search={activeSearch}
              sessionActions={sessionActions}
              sources={computerSources.sources}
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
          <SessionListTotals
            totals={effectiveTotals}
            uncountedNames={multiComputerVisible ? computerSources.uncountedNames : undefined}
            testIdPrefix={testIdPrefix}
          />
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
