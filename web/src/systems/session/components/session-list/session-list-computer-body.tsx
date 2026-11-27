import { profileSessionGroups, projectSessionGroups } from "../../lib/session-list-groups";
import { readCachedOfflineRowsForComputer } from "../../lib/session-offline-catalog";
import { QueryClientContext, type QueryClient } from "@tanstack/react-query";
import { use, useContext } from "react";
import { cn } from "@compozy/ui";

import { isRemoteComputerId } from "@/lib/computer-scope";
import { useOptionalUpgradeSheetActions } from "@/systems/account";
import { useActiveWorkspace, type DataScope } from "@/systems/workspace";

import { SessionListGroupSelectionContext } from "../../contexts/session-list-group-selection-context-value";
import { type ComputerSessionsSource } from "../../hooks/use-computer-sessions-sources";
import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import {
  useSessionListFolderLayout,
  type SessionFolderDialogState,
} from "../../hooks/use-session-list-folder-layout";
import { usePublishSessionListGroupRows } from "../../hooks/use-session-list-group-rows";
import { useSessionListLocalState } from "../../hooks/use-session-list-local-state";
import { scopedCatalogFilters } from "../../hooks/use-session-catalog";
import { useSessionListView, type SessionListViewModel } from "../../hooks/use-session-list-view";
import type { SessionCreateOpenOptions } from "../../lib/session-create-target";
import { sessionListGroupRank } from "../../lib/session-list-group-rows";
import { buildSessionListThreads, threadVisibleOrder } from "../../lib/session-list-threads";
import type { SessionFoldSet, SessionSubagentFolds } from "../../lib/session-sidebar-scope";
import type { SessionThreadBranchChange } from "../../lib/session-thread-branch";
import type { SessionListTarget, WorkspaceFolderPayload } from "../../types";
import { SessionFolderDialogs } from "./session-folder-dialogs";
import { SessionListSingleBody } from "./session-list-body";
import {
  SessionListComputerNote,
  SessionListComputerStateNotice,
} from "./session-list-computer-note";

export interface SessionListComputerBodyProps {
  source: ComputerSessionsSource;
  /** Controlled search string from the parent SessionList. */
  search: string;
  /** True when the window is scoped to this single computer (no section header). */
  scoped: boolean;
  collapsedThreads: SessionFoldSet;
  subagentThreads: SessionSubagentFolds;
  currentSessionId?: string;
  onThreadBranchChange: SessionThreadBranchChange;
  onSelectSession: (session: SessionListTarget) => void;
  onNewSession: (options?: SessionCreateOpenOptions) => void;
  sessionActions: SessionLifecycleActionHandlers;
  testIdPrefix: string;
  onOpenComputerDetails?: (computerId: string) => void;
  /** External create-project-folder trigger from the section header's `+` menu (`F3`). */
  projectFolderDialog?: SessionFolderDialogState<WorkspaceFolderPayload>;
  onProjectFolderDialogChange?: (state: SessionFolderDialogState<WorkspaceFolderPayload>) => void;
  /** Position of this computer section within the window for cross-section selection order. */
  sectionRank?: number;
  /** Active worktree filter forwarded from the parent list view. */
  worktreeId?: string;
}

function isEmptyOnlineComputer(input: {
  filtering: boolean;
  source: ComputerSessionsSource;
  view: SessionListViewModel;
  threadCount: number;
  folderCount: number;
}): boolean {
  const { filtering, source, view, threadCount, folderCount } = input;
  return (
    !filtering &&
    source.read === "ok" &&
    source.count === 0 &&
    !view.groupsLoading &&
    view.groups.length === 0 &&
    threadCount === 0 &&
    folderCount === 0
  );
}

/**
 * One computer's body inside either a Separate section (`F7`) or a Scoped window (`F8`).
 * Non-row states render their status notice without issuing catalog reads (`C7`).
 */
export function SessionListComputerBody(props: SessionListComputerBodyProps) {
  const { source, testIdPrefix, onOpenComputerDetails } = props;
  const upgradeSheet = useOptionalUpgradeSheetActions();

  if (source.read !== "ok" && source.read !== "offline") {
    return (
      <SessionListComputerStateNotice
        computerId={source.key}
        name={source.name}
        onOpenDetails={onOpenComputerDetails ? () => onOpenComputerDetails(source.key) : undefined}
        onRetry={source.retry}
        onUpgrade={upgradeSheet ? () => upgradeSheet.openUpgradeSheet() : undefined}
        read={source.read}
        testIdPrefix={testIdPrefix}
      />
    );
  }

  return <SessionListComputerLiveBody {...props} />;
}

/**
 * Mounted only for `ok` and `offline` computers so each open section owns one
 * `useSessionListView({ computerId })` hook cleanly without loops (`F5`, `F7`).
 */
function SessionListComputerLiveBody({
  source,
  search,
  scoped,
  collapsedThreads,
  subagentThreads,
  currentSessionId,
  onThreadBranchChange,
  onSelectSession,
  onNewSession,
  sessionActions,
  testIdPrefix,
  projectFolderDialog: externalProjectDialog,
  onProjectFolderDialogChange,
  sectionRank = 0,
  worktreeId,
}: SessionListComputerBodyProps) {
  const queryClient = useContext(QueryClientContext);
  const { dataScope } = useActiveWorkspace();
  const parentGroupSelection = use(SessionListGroupSelectionContext);
  const offline = source.read === "offline";
  const readOnly = !source.canOperate || offline;
  const filtering = search.trim() !== "";

  const computerView = useSessionListView({
    scope: dataScope,
    worktreeId,
    computerId: source.key,
    workspaces: source.workspaces,
    matchedWorkspaceIds: source.matchedWorkspaceIds,
    readOnly,
    search,
    multiComputer: !scoped,
    includePinned: false,
    enabled: !offline,
  });

  const { projectFolderDialog: localProjectDialog, setProjectFolderDialog: setLocalProjectDialog } =
    useSessionListLocalState(computerView.selectionKey);
  const projectFolderDialog = externalProjectDialog ?? localProjectDialog;
  const setProjectFolderDialog = onProjectFolderDialogChange ?? setLocalProjectDialog;

  // Offline computers filter rows already cached before disconnect (`Opus 6`, `Astra 15`).
  const listedSessions = computerListRows(queryClient, source, dataScope, computerView);
  const threads = buildSessionListThreads(listedSessions, "");
  const folders = useSessionListFolderLayout(computerView, threads, filtering);
  const grouped = computerView.groupBy !== "none";
  usePublishSessionListGroupRows(
    !grouped && !readOnly ? parentGroupSelection?.rows : undefined,
    `computer:${source.key}`,
    {
      rank: sessionListGroupRank([sectionRank]),
      computerId: source.key,
      sessions: listedSessions,
      visibleOrder: threadVisibleOrder(folders.visibleThreads, collapsedThreads),
      pair: computerView.pair ?? null,
    }
  );

  const handleSelectSession = (session: SessionListTarget) => {
    onSelectSession(
      isRemoteComputerId(source.key) ? { ...session, computerId: source.key } : session
    );
  };
  const handleNewSession = (options?: SessionCreateOpenOptions) => {
    onNewSession({
      ...options,
      computerId: options?.computerId ?? source.key,
      fixedComputer: options?.fixedComputer ?? true,
    });
  };

  const empty = isEmptyOnlineComputer({
    filtering,
    source,
    view: computerView,
    threadCount: threads.length,
    folderCount: folders.sections?.folders.length ?? 0,
  });

  const layout = folders.layout;
  const projectLayout = computerView.projectFolders?.layout;
  const bodyView = offline
    ? offlineComputerListView(computerView, source, dataScope)
    : computerView;

  return (
    <div
      className={cn("flex flex-col gap-0.5", offline && "opacity-70")}
      data-testid={`${testIdPrefix}-computer-${source.key}-body`}
    >
      {empty ? (
        <SessionListComputerNote testId={`${testIdPrefix}-computer-${source.key}-empty`}>
          No sessions on {source.name} yet.
        </SessionListComputerNote>
      ) : (
        <SessionListSingleBody
          collapsedThreads={collapsedThreads}
          currentSessionId={currentSessionId}
          embedded
          filtering={filtering}
          folders={folders}
          normalizedFilter={search.trim().toLocaleLowerCase()}
          onNewSession={handleNewSession}
          onProjectFolderDialog={setProjectFolderDialog}
          onSelectSession={handleSelectSession}
          onThreadBranchChange={onThreadBranchChange}
          pinnedSection={null}
          rankPrefix={[sectionRank]}
          readOnly={readOnly}
          selection={readOnly ? undefined : parentGroupSelection?.selection}
          selectionDrag={readOnly ? undefined : parentGroupSelection?.drag}
          sessionActions={sessionActions}
          subagentThreads={subagentThreads}
          testIdPrefix={`${testIdPrefix}-${source.key}`}
          threads={threads}
          view={bodyView}
        />
      )}
      {layout ? (
        <SessionFolderDialogs
          onClose={() => folders.setDialog(null)}
          onCreate={layout.createFolder}
          onDelete={layout.deleteFolder}
          onRename={layout.renameFolder}
          state={folders.dialog}
        />
      ) : null}
      {projectLayout ? (
        <SessionFolderDialogs
          holds="projects"
          onClose={() => setProjectFolderDialog(null)}
          onCreate={projectLayout.createFolder}
          onDelete={projectLayout.deleteFolder}
          onRename={projectLayout.renameFolder}
          state={projectFolderDialog}
        />
      ) : null}
    </div>
  );
}

function computerListRows(
  queryClient: QueryClient | undefined,
  source: ComputerSessionsSource,
  scope: DataScope | null,
  view: SessionListViewModel
) {
  if (source.read !== "offline") return view.catalog?.sessions ?? [];
  if (scope === null) return [];
  const filters = scopedCatalogFilters(
    scope,
    view.groupFilters,
    { all_profiles: true },
    source.key,
    source.matchedWorkspaceIds
  );
  return filters.emptyRemoteMatch
    ? []
    : readCachedOfflineRowsForComputer(queryClient, source.key, filters.scoped);
}

function offlineComputerListView(
  view: SessionListViewModel,
  source: ComputerSessionsSource,
  scope: DataScope | null
): SessionListViewModel {
  if (scope === null || !source.facets) return { ...view, groupBy: "none" };
  const groupInput = { computerId: source.key, offline: true };
  const groups =
    view.groupBy === "project"
      ? projectSessionGroups(source.facets, {
          ...groupInput,
          workspaces: source.workspaces,
          lens: { all_profiles: true },
        })
      : view.groupBy === "profile"
        ? profileSessionGroups(source.facets, {
            ...groupInput,
            scope,
            profiles: source.profiles ?? [],
            matchedWorkspaceIds: source.matchedWorkspaceIds,
          })
        : [];
  return groups.length > 0 ? { ...view, groups } : { ...view, groupBy: "none" };
}
