import {
  dataScopeForFocus,
  desktopPartitionFor,
  USER_FOCUS,
  type BreadthMode,
  type DataScope,
  type Focus,
} from "./destination";
import { DESTINATION_COPY } from "./workspace-scope-copy";
import type { WorkspacePayload } from "../types";

export interface WorkspaceChipIdentity {
  /** Which destination the chip names; `all` draws the stacked mark. */
  kind: "all" | "user" | "project";
  name: string;
  monogram: string;
}

export interface ActiveWorkspaceResolution {
  /** Effective breadth after resolution. */
  breadthMode: BreadthMode;
  /** Effective focus: a missing remembered project resolves to User. */
  focus: Focus;
  /**
   * What every surface lists. `null` only while unsettled (a stored project
   * focus not yet confirmed by the catalog) — never User, never the aggregate.
   */
  dataScope: DataScope | null;
  /** True while the workspace catalog is still unknown — nothing below is settled. */
  pending: boolean;
  projectWorkspaces: WorkspacePayload[];
  registeredWorkspaces: WorkspacePayload[];
  rememberedWorkspace: WorkspacePayload | undefined;
  /** The focused project while it is the data scope; never set under User or All projects. */
  activeWorkspace: WorkspacePayload | undefined;
  activeWorkspaceId: string | null;
  /** Project-only data binding: `null` under User and under All projects. */
  runtimeWorkspace: WorkspacePayload | undefined;
  runtimeWorkspaceId: string | null;
  /** Layout binding: the focused partition (`user` for User). */
  desktopWorkspace: WorkspacePayload | undefined;
  desktopWorkspaceId: string | null;
  chip: WorkspaceChipIdentity;
}

export function workspaceMonogram(name: string): string {
  return name.trim().slice(0, 2).toUpperCase() || "WS";
}

const ALL_PROJECTS_CHIP: WorkspaceChipIdentity = {
  kind: "all",
  name: DESTINATION_COPY.allProjects,
  monogram: DESTINATION_COPY.allProjectsMonogram,
};

const USER_CHIP: WorkspaceChipIdentity = {
  kind: "user",
  name: DESTINATION_COPY.user,
  monogram: DESTINATION_COPY.userMonogram,
};

function chipFor(
  breadthMode: BreadthMode,
  focus: Focus,
  workspace: WorkspacePayload | undefined
): WorkspaceChipIdentity {
  if (breadthMode === "all") return ALL_PROJECTS_CHIP;
  if (focus.kind === "user" || !workspace) return USER_CHIP;
  return { kind: "project", name: workspace.name, monogram: workspaceMonogram(workspace.name) };
}

/**
 * The destination model: the globe picks breadth, the switcher picks focus.
 * The desktop follows the focus (`user` for User); under All projects it stays
 * on the focused project's partition, or `global` when User is focused, until
 * All projects gets its own desktop.
 */
export function resolveActiveWorkspace(input: {
  workspaces: readonly WorkspacePayload[];
  breadthMode: BreadthMode;
  focus: Focus;
  /** Pass true while the workspace catalog is unsettled; the resolver then claims nothing. */
  pending?: boolean;
}): ActiveWorkspaceResolution {
  const registeredWorkspaces = [...input.workspaces];
  if (input.pending) {
    return {
      breadthMode: input.breadthMode,
      focus: input.focus,
      // All projects and User are always valid destinations; a project is not
      // one until the catalog confirms it still exists.
      dataScope:
        input.breadthMode === "all"
          ? { kind: "all" }
          : input.focus.kind === "user"
            ? { kind: "user" }
            : null,
      pending: true,
      projectWorkspaces: [],
      registeredWorkspaces,
      rememberedWorkspace: undefined,
      activeWorkspace: undefined,
      activeWorkspaceId: null,
      runtimeWorkspace: undefined,
      runtimeWorkspaceId: null,
      desktopWorkspace: undefined,
      desktopWorkspaceId: null,
      chip:
        input.breadthMode === "all"
          ? ALL_PROJECTS_CHIP
          : { kind: input.focus.kind, name: "", monogram: "" },
    };
  }
  const projectWorkspaces = registeredWorkspaces;
  const requestedFocus = input.focus;
  const rememberedWorkspace =
    requestedFocus.kind === "project"
      ? projectWorkspaces.find(workspace => workspace.id === requestedFocus.workspaceId)
      : undefined;
  // A deleted project is not a destination; User always is.
  const focus: Focus = rememberedWorkspace ? requestedFocus : USER_FOCUS;
  const breadthMode = input.breadthMode;
  const focused = breadthMode === "focused";
  const activeWorkspace = focused ? rememberedWorkspace : undefined;
  const dataScope: DataScope = focused ? dataScopeForFocus(focus) : { kind: "all" };
  const desktopWorkspaceId = desktopPartitionFor(breadthMode, focus);

  return {
    breadthMode,
    focus,
    dataScope,
    pending: false,
    projectWorkspaces,
    registeredWorkspaces,
    rememberedWorkspace,
    activeWorkspace,
    activeWorkspaceId: activeWorkspace?.id ?? null,
    runtimeWorkspace: activeWorkspace,
    runtimeWorkspaceId: activeWorkspace?.id ?? null,
    desktopWorkspace: rememberedWorkspace,
    desktopWorkspaceId,
    chip: chipFor(breadthMode, focus, activeWorkspace),
  };
}
