export { AppShellContainer } from "./components/app-shell-container";
export { AppShellLayout } from "./components/app-shell-layout";
export { AppShellBoundary, AppShellErrorBoundary } from "./components/app-shell-boundary";
export { WorkspaceOnboarding } from "./components/workspace-onboarding";
export { WorkspacePicker } from "./components/workspace-picker";
export { useActiveWorkspace } from "./hooks/use-active-workspace";
export { useWorkspaceEvents } from "./hooks/use-workspace-events";
export { useResolveWorkspace, useWorkspaces } from "./hooks/use-workspaces";
export {
  useActiveWorkspaceContext,
  ActiveWorkspaceContext,
  type ActiveWorkspaceContextValue,
} from "./lib/active-workspace-context";
export { ACTIVE_WORKSPACE_HEADER, workspaceHeaders } from "./lib/workspace-headers";
export { buildWorkspaceSocketUrl } from "./lib/workspace-events";
export { workspaceKeys } from "./lib/query-keys";
export type { Workspace } from "./types";
export {
  resetActiveWorkspaceStoreForTests,
  useActiveWorkspaceStore,
} from "./stores/active-workspace-store";
