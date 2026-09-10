import { partitionProjectWorkspaces } from "./project-workspaces";
import { GLOBAL_SCOPE_COPY } from "./workspace-scope-copy";
import type { WorkspaceScopeMode } from "./workspace-scope-mode";
import type { WorkspacePayload } from "../types";

export interface WorkspaceChipIdentity {
  name: string;
  monogram: string;
}

/** Reserved daemon partition for profile-owned desktop state without a project workspace. */
export const GLOBAL_DESKTOP_WORKSPACE_ID = "global";

export interface ActiveWorkspaceResolution {
  /** Effective destination after zero-project and missing-id rules. */
  scope: WorkspaceScopeMode;
  /** True while the workspace catalog or `$HOME` is still unknown — nothing below is settled. */
  pending: boolean;
  projectWorkspaces: WorkspacePayload[];
  registeredWorkspaces: WorkspacePayload[];
  homeWorkspace: WorkspacePayload | undefined;
  rememberedWorkspace: WorkspacePayload | undefined;
  /** Project workspace when `scope` is workspace; never the home row. */
  activeWorkspace: WorkspacePayload | undefined;
  activeWorkspaceId: string | null;
  /** Data binding: selected project in workspace scope; no workspace in Global. */
  runtimeWorkspace: WorkspacePayload | undefined;
  runtimeWorkspaceId: string | null;
  /** Layout binding: remembered project, retained desktop, or initial fallback. */
  desktopWorkspace: WorkspacePayload | undefined;
  desktopWorkspaceId: string | null;
  chip: WorkspaceChipIdentity;
  toggleLocked: boolean;
  canDisableGlobal: boolean;
}

export function workspaceMonogram(name: string): string {
  return name.trim().slice(0, 2).toUpperCase() || "WS";
}

export function resolveActiveWorkspace(input: {
  workspaces: readonly WorkspacePayload[];
  userHomeDir: string | undefined;
  scope: WorkspaceScopeMode;
  selectedWorkspaceId: string | null;
  desktopWorkspaceId?: string | null;
  /** Pass true while either source query is unsettled; the resolver then claims nothing. */
  pending?: boolean;
}): ActiveWorkspaceResolution {
  const registeredWorkspaces = [...input.workspaces];
  if (input.pending) {
    return {
      scope: input.scope,
      pending: true,
      projectWorkspaces: [],
      registeredWorkspaces,
      homeWorkspace: undefined,
      rememberedWorkspace: undefined,
      activeWorkspace: undefined,
      activeWorkspaceId: null,
      runtimeWorkspace: undefined,
      runtimeWorkspaceId: null,
      desktopWorkspace: undefined,
      desktopWorkspaceId: null,
      chip:
        input.scope === "global"
          ? { name: GLOBAL_SCOPE_COPY.chipLabel, monogram: GLOBAL_SCOPE_COPY.chipMonogram }
          : { name: "", monogram: "" },
      toggleLocked: true,
      canDisableGlobal: false,
    };
  }
  const { homeWorkspace, projectWorkspaces } = partitionProjectWorkspaces(
    registeredWorkspaces,
    input.userHomeDir
  );
  const rememberedWorkspace = input.selectedWorkspaceId
    ? projectWorkspaces.find(workspace => workspace.id === input.selectedWorkspaceId)
    : undefined;
  const toggleLocked = projectWorkspaces.length === 0;
  const canDisableGlobal = rememberedWorkspace !== undefined && !toggleLocked;

  let scope: WorkspaceScopeMode = input.scope;
  if (toggleLocked || (input.scope === "workspace" && rememberedWorkspace === undefined)) {
    scope = "global";
  }

  const activeWorkspace = scope === "workspace" ? rememberedWorkspace : undefined;
  const runtimeWorkspace = activeWorkspace;
  const retainedDesktopWorkspace = projectWorkspaces.find(
    workspace => workspace.id === input.desktopWorkspaceId
  );
  // Preserve the layout independently of Global's unscoped data destination.
  const desktopWorkspace =
    rememberedWorkspace ??
    retainedDesktopWorkspace ??
    (input.desktopWorkspaceId === GLOBAL_DESKTOP_WORKSPACE_ID ? undefined : projectWorkspaces[0]);
  const desktopWorkspaceId = desktopWorkspace?.id ?? GLOBAL_DESKTOP_WORKSPACE_ID;

  return {
    scope,
    pending: false,
    projectWorkspaces,
    registeredWorkspaces,
    homeWorkspace,
    rememberedWorkspace,
    activeWorkspace,
    activeWorkspaceId: activeWorkspace?.id ?? null,
    runtimeWorkspace,
    runtimeWorkspaceId: runtimeWorkspace?.id ?? null,
    desktopWorkspace,
    desktopWorkspaceId,
    chip:
      scope === "global"
        ? { name: GLOBAL_SCOPE_COPY.chipLabel, monogram: GLOBAL_SCOPE_COPY.chipMonogram }
        : {
            name: activeWorkspace?.name ?? GLOBAL_SCOPE_COPY.chipLabel,
            monogram: workspaceMonogram(activeWorkspace?.name ?? ""),
          },
    toggleLocked,
    canDisableGlobal,
  };
}
