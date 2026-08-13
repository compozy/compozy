import { partitionProjectWorkspaces } from "./project-workspaces";
import { GLOBAL_SCOPE_COPY } from "./workspace-scope-copy";
import type { WorkspaceScopeMode } from "./workspace-scope-mode";
import type { WorkspacePayload } from "../types";

export interface WorkspaceChipIdentity {
  name: string;
  monogram: string;
}

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
  /** Daemon binding: home row when Global, else the selected project. */
  runtimeWorkspace: WorkspacePayload | undefined;
  runtimeWorkspaceId: string | null;
  chip: WorkspaceChipIdentity;
  toggleLocked: boolean;
  canDisableGlobal: boolean;
  deletionNotice: string | null;
}

export function workspaceMonogram(name: string): string {
  return name.trim().slice(0, 2).toUpperCase() || "WS";
}

export function resolveActiveWorkspace(input: {
  workspaces: readonly WorkspacePayload[];
  userHomeDir: string | undefined;
  scope: WorkspaceScopeMode;
  selectedWorkspaceId: string | null;
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
      chip:
        input.scope === "global"
          ? { name: GLOBAL_SCOPE_COPY.chipLabel, monogram: GLOBAL_SCOPE_COPY.chipMonogram }
          : { name: "", monogram: "" },
      toggleLocked: true,
      canDisableGlobal: false,
      deletionNotice: null,
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
  const rememberedMissing = Boolean(input.selectedWorkspaceId) && rememberedWorkspace === undefined;
  const canDisableGlobal = rememberedWorkspace !== undefined && !toggleLocked;

  let scope: WorkspaceScopeMode = input.scope;
  if (toggleLocked || (input.scope === "workspace" && rememberedWorkspace === undefined)) {
    scope = "global";
  }

  const activeWorkspace = scope === "workspace" ? rememberedWorkspace : undefined;
  const runtimeWorkspace = scope === "global" ? homeWorkspace : activeWorkspace;
  const deletionNotice =
    scope === "global" && rememberedMissing ? GLOBAL_SCOPE_COPY.rememberedWorkspaceRemoved : null;

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
    chip:
      scope === "global"
        ? { name: GLOBAL_SCOPE_COPY.chipLabel, monogram: GLOBAL_SCOPE_COPY.chipMonogram }
        : {
            name: activeWorkspace?.name ?? GLOBAL_SCOPE_COPY.chipLabel,
            monogram: workspaceMonogram(activeWorkspace?.name ?? ""),
          },
    toggleLocked,
    canDisableGlobal,
    deletionNotice,
  };
}
