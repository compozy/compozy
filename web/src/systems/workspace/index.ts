// Types
export type {
  SessionProviderOption,
  WorkspaceDetailPayload,
  WorkspacePayload,
  WorkspaceResponse,
  WorkspacesResponse,
} from "./types";
export type {
  DiscoveredWorktreePayload,
  RunWorktreeExitActionParams,
  WorktreeCatalogEventPayload,
  WorktreeDisplayState,
  WorktreeExitAction,
  WorktreeExitActionPlan,
  WorktreeExitCleanupEvidence,
  WorktreeExitCommitScope,
  WorktreeExitPlanPayload,
  WorktreeForgeCapabilities,
  WorktreeForgeStatus,
  WorktreeGitStatus,
  WorktreeInspectionPayload,
  WorktreePayload,
  WorktreeRepoState,
  WorktreesResponse,
  WorktreeStatusPayload,
} from "./types";

// Adapters
export {
  createWorkspace,
  deleteWorkspace,
  fetchWorkspace,
  fetchWorkspaces,
  resolveWorkspace,
} from "./adapters/workspace-api";
export type { CreateWorkspaceParams, ResolveWorkspaceParams } from "./adapters/workspace-api";

// Query infrastructure
export { isOperatorHomeWorkspace, partitionProjectWorkspaces } from "./lib/project-workspaces";
export type { ProjectWorkspacePartition, WorkspaceHomeCandidate } from "./lib/project-workspaces";
export { resolveActiveWorkspace, workspaceMonogram } from "./lib/active-workspace";
export type { ActiveWorkspaceResolution, WorkspaceChipIdentity } from "./lib/active-workspace";
export { toWorkspaceCommandSelectOptions } from "./lib/workspace-command-select-options";
export type { WorkspaceScopeMode } from "./lib/workspace-scope-mode";
export { workspaceKeys } from "./lib/query-keys";
export {
  workspaceDetailOptions,
  worktreeDetailOptions,
  worktreeExitPlanOptions,
  worktreesListOptions,
  workspacesListOptions,
  worktreeStatusOptions,
} from "./lib/query-options";
export {
  GLOBAL_SCOPE_COPY,
  destinationLabel,
  globalScopeTooltipOn,
} from "./lib/workspace-scope-copy";

// Worktree data layer
export {
  adoptWorktree,
  cancelWorktreeCreate,
  createWorktree,
  dismissWorktree,
  fetchWorktrees,
  removeWorktree,
  WorktreeApiError,
} from "./adapters/worktree-api";
export type { AdoptWorktreeParams, CreateWorktreeParams } from "./adapters/worktree-api";
export {
  cancelWorktreeExitAction,
  fetchWorktreeExitPlan,
  fetchWorktreeStatus,
  inspectWorktree,
  runWorktreeExitAction,
} from "./adapters/worktree-exit-api";
export type { WorktreeStatusQuery } from "./adapters/worktree-exit-api";
export {
  commitScopeTotal,
  isCommitScopeEmpty,
  toWorktreeExitLadder,
} from "./lib/worktree-exit-ladder";
export type { WorktreeExitLadder, WorktreeExitLadderRow } from "./lib/worktree-exit-ladder";
export {
  isWorktreeExitProgressTerminal,
  reduceWorktreeExitEvent,
} from "./lib/worktree-exit-progress";
export type {
  WorktreeExitHookChunk,
  WorktreeExitPhaseState,
  WorktreeExitProgress,
  WorktreeExitProgressPhase,
  WorktreeExitProgressStatus,
} from "./lib/worktree-exit-progress";
export {
  parseWorktreeExitEvent,
  WORKTREE_EXIT_EVENTS,
  WORKTREE_EXIT_TERMINAL_EVENTS,
  WORKTREE_LIFECYCLE_EVENTS,
} from "./lib/worktree-events";
export type {
  WorktreeExitCTA,
  WorktreeExitEventName,
  WorktreeExitEventPayload,
  WorktreeExitPhase,
  WorktreeExitStepResult,
  WorktreeExitStream,
  WorktreeStreamEventName,
} from "./lib/worktree-events";
export { reconcileWorktreeList, removeWorktreeFromList } from "./lib/worktree-list-reconciliation";
export { toWorktreeDisplayState, toWorktreeNestEntries } from "./lib/worktree-display";
export type { WorktreeNestEntry } from "./lib/worktree-display";
export {
  adoptedWorktreeCount,
  runningAgentCount,
  sortWorktreeNestEntries,
  truncateWorktreeNest,
  WORKTREE_NEST_VISIBLE_LIMIT,
} from "./lib/worktree-sort";
export type { TruncatedNest } from "./lib/worktree-sort";
export {
  decodeWorktreeRefusal,
  refusalBranchHolder,
  WORKTREE_ERROR_CODES,
} from "./lib/worktree-refusal";
export type { WorktreeRefusal, WorktreeRemovalRisk } from "./lib/worktree-refusal";
export { groupWorkspaceTree } from "./lib/workspace-tree";
export type { WorkspaceTreeNode, WorktreeListingByWorkspace } from "./lib/workspace-tree";

// Hooks
export { useActiveWorkspace } from "./hooks/use-active-workspace";
export {
  activeWorkspaceStore,
  isActiveWorkspaceStoreHydrated,
  rehydrateActiveWorkspaceStore,
  useSelectedWorkspaceId,
  useWorkspaceScopeMode,
} from "./hooks/use-active-workspace-store";
export { useUserHomeDir } from "./hooks/use-user-home-dir";
export {
  ACTIVE_WORKSPACE_PERSIST_KEY,
  clearActiveWorkspaceSelection,
  clearActiveWorktreeSelection,
  disableGlobalScope,
  enableGlobalScope,
  pruneWorktreeScopes,
  setActiveWorkspaceId,
  setActiveWorktreeId,
  SHELL_WORKTREE_SCOPE,
  toggleGlobalScope,
} from "./stores/active-workspace-store";
export {
  useCreateWorkspace,
  useDeleteWorkspace,
  useResolveWorkspace,
  useWorkspace,
  useWorkspaces,
} from "./hooks/use-workspaces";
export {
  clearWorktreeForScope,
  selectWorktreeForScope,
  useActiveWorktree,
  useScopedWorktreeFilter,
  useSelectedWorktreeId,
} from "./hooks/use-active-worktree";
export type { ActiveWorktreeSelection, WorktreeFallbackReason } from "./hooks/use-active-worktree";
export {
  useAdoptWorktree,
  useCancelWorktreeCreate,
  useCreateWorktree,
  useDismissWorktree,
  useRemoveWorktree,
  useWorktreeListings,
  useWorktrees,
} from "./hooks/use-worktrees";
export { useWorktreeCatalogStream } from "./hooks/use-worktree-catalog-stream";
export type {
  WorktreeCatalogEventSource,
  WorktreeCatalogEventSourceFactory,
  WorktreeCatalogStreamStatus,
} from "./hooks/use-worktree-catalog-stream";
export {
  invalidateWorktreeExitScope,
  useCancelWorktreeExitAction,
  useRefreshWorktreeStatus,
  useRunWorktreeExitAction,
  useWorktreeDetail,
  useWorktreeExitPlan,
  useWorktreeStatus,
} from "./hooks/use-worktree-exit";
export { useWorktreeMaterialization } from "./hooks/use-worktree-materialization";
export type {
  WorktreeMaterialization,
  WorktreeMaterializationStatus,
} from "./hooks/use-worktree-materialization";
export { useWorktreeExitLadder } from "./hooks/use-worktree-exit-ladder";
export type {
  UseWorktreeExitLadderOptions,
  WorktreeExitLadderModel,
} from "./hooks/use-worktree-exit-ladder";
export { useWorktreeStream, worktreeStreamURL } from "./hooks/use-worktree-stream";
export type {
  WorktreeEventSource,
  WorktreeEventSourceFactory,
  WorktreeExitEventHandler,
  WorktreeStreamStatus,
} from "./hooks/use-worktree-stream";

// Components
export { OptionCard } from "./components/option-card";
export type {
  OptionCardHeaderProps,
  OptionCardIconProps,
  OptionCardProps,
  OptionCardSize,
  OptionCardTone,
} from "./components/option-card";
export {
  WorkspaceCommandSelect,
  type WorkspaceCommandSelectOption,
  type WorkspaceCommandSelectProps,
} from "./components/workspace-command-select";
export { WorkspaceScopeStatement } from "./components/workspace-scope-statement";
export type {
  WorkspaceScopeStatementKind,
  WorkspaceScopeStatementProps,
  WorkspaceScopeStatementVariant,
} from "./components/workspace-scope-statement";
export { useCreateDestination } from "./hooks/use-create-destination";
export type { CreateDestination } from "./hooks/use-create-destination";
export { WorktreeRefSelect } from "./components/worktree-ref-select";
export type { WorktreeRefSelectProps } from "./components/worktree-ref-select";
export { WorktreeStateChip } from "./components/worktree-state-chip";
export type { WorktreeChipState, WorktreeStateChipProps } from "./components/worktree-state-chip";
export { WorktreeStateDot } from "./components/worktree-state-dot";
export type { WorktreeDotState, WorktreeStateDotProps } from "./components/worktree-state-dot";
export { WorktreeSignals } from "./components/worktree-signals";
export type { WorktreeSignalsProps } from "./components/worktree-signals";
export { WorktreeOriginSignal } from "./components/worktree-signal-parts";
export { WorktreeRow } from "./components/worktree-row";
export type { WorktreeRowProps } from "./components/worktree-row";
export { WorktreeCreateDialog } from "./components/worktree-create-dialog";
export type { WorktreeCreateDialogProps } from "./components/worktree-create-dialog";
export { useWorktreeCreateDialog } from "./hooks/use-worktree-create-dialog";
export type {
  WorktreeCreateDialogModel,
  WorktreeCreateDraft,
} from "./hooks/use-worktree-create-dialog";
export { WorktreeAdoptDialog } from "./components/worktree-adopt-dialog";
export type { WorktreeAdoptDialogProps } from "./components/worktree-adopt-dialog";
export { WorktreeRemoveDialog } from "./components/worktree-remove-dialog";
export type { WorktreeRemoveDialogProps } from "./components/worktree-remove-dialog";
export { WorktreeMissingResolutionDialog } from "./components/worktree-missing-resolution-dialog";
export type { WorktreeMissingResolutionDialogProps } from "./components/worktree-missing-resolution-dialog";
export {
  buildWorktreeCreatePreview,
  deriveWorktreeBranch,
  deriveWorktreeParentDir,
  sanitizeWorktreeName,
} from "./lib/worktree-naming";
export { WorktreeAggregate, WorktreeNestList } from "./components/worktree-nest-list";
export type {
  WorktreeAggregateProps,
  WorktreeNestListProps,
} from "./components/worktree-nest-list";
export { WorkspaceSetupDialog } from "./components/workspace-setup";
export type { WorkspaceSetupModel } from "./components/workspace-setup";
export { useWorkspaceSetupContent } from "./hooks/use-workspace-setup-content";
export type { WorkspaceSetupContent } from "./hooks/use-workspace-setup-content";
export type {
  WorkspaceSetupCollection,
  WorkspaceSetupDefaultsModel,
  WorkspaceSetupSandboxOption,
} from "./lib/workspace-setup-defaults";

// Assisted exit surface (Task 07)
export { WorktreeDetailDialog } from "./components/worktree-detail-dialog";
export { WorktreeExitProgressSurface } from "./components/worktree-exit-progress-surface";
export { WorktreeDetailHeader } from "./components/worktree-detail-header";
export { WorktreeStatusStrip } from "./components/worktree-status-strip";
export { WorktreeExitControl } from "./components/worktree-exit-control";
export { AGENT_MESSAGE_PROMPT, WorktreeCommitDialog } from "./components/worktree-commit-dialog";
export { WorktreeScopeBlock } from "./components/worktree-scope-block";
export { WorktreePrDialog } from "./components/worktree-pr-dialog";
export { WorktreePrActionRows } from "./components/worktree-pr-action-rows";
export type {
  WorktreePrActionKey,
  WorktreePrActionRow,
} from "./components/worktree-pr-action-rows";
export { WorktreeExitProgress as WorktreeExitProgressToast } from "./components/worktree-exit-progress";
export { WorktreePhaseSteps } from "./components/worktree-phase-steps";
export { WorktreeMergedEvidence } from "./components/worktree-merged-evidence";
export { useWorktreeDetailContext } from "./hooks/use-worktree-detail-context";
export type { WorktreeDetailModel } from "./hooks/use-worktree-detail-context";
export { toWorktreePrDialogModel } from "./lib/worktree-pr-rows";
export type { WorktreePrDialogMode, WorktreePrDialogModel } from "./lib/worktree-pr-rows";
