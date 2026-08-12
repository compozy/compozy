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
  WorktreeCatalogEventPayload,
  WorktreeDisplayState,
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
  worktreesListOptions,
  workspacesListOptions,
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
export { WorktreeStateChip } from "./components/worktree-state-chip";
export type { WorktreeChipState, WorktreeStateChipProps } from "./components/worktree-state-chip";
export { WorktreeStateDot } from "./components/worktree-state-dot";
export type { WorktreeDotState, WorktreeStateDotProps } from "./components/worktree-state-dot";
export { WorktreeSignals } from "./components/worktree-signals";
export type { WorktreeSignalsProps } from "./components/worktree-signals";
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
