import {
  allProjectsTooltipOn,
  DESTINATION_COPY,
  focusLabel,
  type ActiveWorktreeSelection,
  type BreadthMode,
  type Focus,
  type WorkspaceChipIdentity,
  type WorkspacePayload,
} from "@/systems/workspace";

interface DesktopMenubarScopeInput {
  activeWorkspace: WorkspacePayload | undefined;
  breadthMode: BreadthMode;
  chip?: WorkspaceChipIdentity;
  /** The focus turning All projects off returns to. */
  focus: Focus;
  /** Name of the focused project; ignored for User. */
  focusWorkspaceName?: string | null;
  scopePending: boolean;
  worktreeSelection?: ActiveWorktreeSelection;
}

function fallbackChip(
  breadthMode: BreadthMode,
  activeWorkspace: WorkspacePayload | undefined
): WorkspaceChipIdentity {
  if (breadthMode === "all") {
    return { kind: "all", name: DESTINATION_COPY.allProjects };
  }
  if (!activeWorkspace) {
    return { kind: "user", name: DESTINATION_COPY.user };
  }
  return { kind: "project", name: activeWorkspace.name };
}

/**
 * Topbar destination model. The globe is never locked by the catalog: User is
 * always a focus, so turning All projects off always has somewhere to go. It
 * only waits while resolution is pending.
 */
export function desktopMenubarScopeModel({
  activeWorkspace,
  breadthMode,
  chip,
  focus,
  focusWorkspaceName,
  scopePending,
  worktreeSelection,
}: DesktopMenubarScopeInput) {
  const allProjectsOn = breadthMode === "all";
  const workspace = chip ?? fallbackChip(breadthMode, activeWorkspace);
  const toggleTooltip =
    !scopePending && allProjectsOn
      ? allProjectsTooltipOn(focusLabel(focus, focusWorkspaceName ?? activeWorkspace?.name))
      : DESTINATION_COPY.allProjects;
  const projectFocused = !allProjectsOn && focus.kind === "project";
  const fallback = projectFocused ? (worktreeSelection?.fallback ?? null) : null;
  const fallbackNotice = fallback
    ? `${fallback.name ?? "The selected worktree"} isn't available, so new sessions start in the main project folder.`
    : null;

  return {
    fallback,
    fallbackNotice,
    allProjectsOn,
    toggleTooltip,
    workspace: {
      ...workspace,
      worktree: projectFocused ? (worktreeSelection?.activeWorktree?.name ?? null) : null,
    },
  };
}
