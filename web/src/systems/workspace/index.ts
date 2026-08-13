// Types
export type {
  SessionProviderOption,
  WorkspaceDetailPayload,
  WorkspacePayload,
  WorkspaceResponse,
  WorkspacesResponse,
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
export { workspaceDetailOptions, workspacesListOptions } from "./lib/query-options";
export {
  GLOBAL_SCOPE_COPY,
  destinationLabel,
  globalScopeTooltipOn,
} from "./lib/workspace-scope-copy";

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
  disableGlobalScope,
  enableGlobalScope,
  setActiveWorkspaceId,
  toggleGlobalScope,
} from "./stores/active-workspace-store";
export {
  useCreateWorkspace,
  useDeleteWorkspace,
  useResolveWorkspace,
  useWorkspace,
  useWorkspaces,
} from "./hooks/use-workspaces";

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
export { WorkspaceSetupDialog } from "./components/workspace-setup";
export type { WorkspaceSetupModel } from "./components/workspace-setup";
export { useWorkspaceSetupContent } from "./hooks/use-workspace-setup-content";
export type { WorkspaceSetupContent } from "./hooks/use-workspace-setup-content";
export type {
  WorkspaceSetupCollection,
  WorkspaceSetupDefaultsModel,
  WorkspaceSetupSandboxOption,
} from "./lib/workspace-setup-defaults";
