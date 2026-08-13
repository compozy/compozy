import { useActiveWorkspace } from "./use-active-workspace";
import { GLOBAL_SCOPE_COPY } from "../lib/workspace-scope-copy";
import type { WorkspaceScopeMode } from "../lib/workspace-scope-mode";

export interface CreateDestination {
  scope: WorkspaceScopeMode;
  /** True while scope resolution is unsettled — withhold submits that bind a destination. */
  pending: boolean;
  workspaceId: string | null;
  workspaceName: string | null;
  destinationLabel: string;
  userHomeDir: string | undefined;
  homeWorkspaceId: string | undefined;
  runtimeWorkspaceId: string | null;
  /** Session root or operator-home path for `kind="session"` statements. */
  sessionRoot: string;
}

/** Derived create/install landing from the menubar-owned scope. */
export function useCreateDestination(): CreateDestination {
  const active = useActiveWorkspace();
  const isGlobal = active.scope === "global";
  const workspaceName = active.activeWorkspace?.name ?? "";

  return {
    scope: active.scope,
    pending: active.pending,
    workspaceId: isGlobal ? null : active.activeWorkspaceId,
    workspaceName: isGlobal ? null : workspaceName || null,
    destinationLabel: isGlobal ? GLOBAL_SCOPE_COPY.chipLabel : workspaceName,
    userHomeDir: active.userHomeDir,
    homeWorkspaceId: active.homeWorkspace?.id,
    runtimeWorkspaceId: active.runtimeWorkspaceId,
    sessionRoot: isGlobal ? (active.userHomeDir ?? "~") : (active.activeWorkspace?.root_dir ?? ""),
  };
}
