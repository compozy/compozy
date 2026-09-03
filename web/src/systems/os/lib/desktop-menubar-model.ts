import {
  GLOBAL_SCOPE_COPY,
  globalScopeTooltipOn,
  workspaceMonogram,
  type ActiveWorktreeSelection,
  type WorkspaceChipIdentity,
  type WorkspacePayload,
  type WorkspaceScopeMode,
} from "@/systems/workspace";

interface DesktopMenubarScopeInput {
  activeWorkspace: WorkspacePayload | undefined;
  canDisableGlobal: boolean;
  chip?: WorkspaceChipIdentity;
  rememberedWorkspaceName?: string | null;
  scope: WorkspaceScopeMode;
  scopePending: boolean;
  toggleLocked: boolean;
  worktreeSelection?: ActiveWorktreeSelection;
}

export function desktopMenubarScopeModel({
  activeWorkspace,
  canDisableGlobal,
  chip,
  rememberedWorkspaceName,
  scope,
  scopePending,
  toggleLocked,
  worktreeSelection,
}: DesktopMenubarScopeInput) {
  const globalOn = scope === "global";
  const rememberedName =
    rememberedWorkspaceName === undefined
      ? (activeWorkspace?.name ?? null)
      : rememberedWorkspaceName;
  const workspace =
    chip ??
    (activeWorkspace
      ? { name: activeWorkspace.name, monogram: workspaceMonogram(activeWorkspace.name) }
      : { name: GLOBAL_SCOPE_COPY.chipLabel, monogram: GLOBAL_SCOPE_COPY.chipMonogram });
  const toggleTooltip = scopePending
    ? GLOBAL_SCOPE_COPY.tooltipOff
    : toggleLocked
      ? GLOBAL_SCOPE_COPY.tooltipLocked
      : globalOn
        ? rememberedName
          ? globalScopeTooltipOn(rememberedName)
          : GLOBAL_SCOPE_COPY.tooltipPickWorkspace
        : GLOBAL_SCOPE_COPY.tooltipOff;
  const toggleLockedReason = scopePending
    ? undefined
    : toggleLocked
      ? GLOBAL_SCOPE_COPY.tooltipLocked
      : globalOn && !canDisableGlobal
        ? GLOBAL_SCOPE_COPY.tooltipPickWorkspace
        : undefined;
  const fallback = globalOn ? null : (worktreeSelection?.fallback ?? null);
  const fallbackNotice = fallback
    ? `${fallback.name ?? "The selected worktree"} is ${fallback.reason === "missing" ? "missing" : "unavailable"} — new work runs at the workspace root`
    : null;

  return {
    fallback,
    fallbackNotice,
    globalOn,
    toggleLockedReason,
    toggleTooltip,
    workspace: {
      ...workspace,
      worktree: globalOn ? null : (worktreeSelection?.activeWorktree?.name ?? null),
    },
  };
}
