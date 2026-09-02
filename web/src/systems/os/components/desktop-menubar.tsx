import { Popover, PopoverContent, PopoverTrigger } from "@compozy/ui";

import { usePaletteCommand } from "../hooks/use-palette-registry";
import type { OsAttentionModel } from "../hooks/use-os-attention";
import type { DesktopOverlay } from "../hooks/use-desktop-overlays";
import { useMenubarActions } from "../hooks/use-menubar-actions";
import { useMenubarAttentionSelection } from "../hooks/use-menubar-attention-selection";
import { useDesktop } from "../hooks/use-desktop";
import { desktopMenubarScopeModel } from "../lib/desktop-menubar-model";
import { OsHydrationStatus } from "./os-hydration-status";
import { OsMenuBar } from "./os-menubar";
import { AttentionBell } from "./attention-bell";
import { MenubarUpdateIndicator } from "./menubar/menubar-update-indicator";
import { CompozyMenu } from "./menubar/compozy-menu";
import { GoMenu } from "./menubar/go-menu";
import { HelpMenu } from "./menubar/help-menu";
import { SessionMenu } from "./menubar/session-menu";
import { WindowMenu } from "./menubar/window-menu";
import { WorkspaceMenu } from "./menubar/workspace-menu";
import {
  workspaceMonogram,
  type ActiveWorktreeSelection,
  type WorkspaceChipIdentity,
  type WorkspacePayload,
  type WorkspaceScopeMode,
  type WorktreeListingByWorkspace,
  type WorktreeNestEntry,
} from "@/systems/workspace";

import { GlobalScopeToggle } from "./global-scope-toggle";

export interface DesktopMenubarProps {
  /** Extra classes on the bar itself (first-run dimming). */
  className?: string;
  workspaces: WorkspacePayload[];
  activeWorkspace: WorkspacePayload | undefined;
  chip?: WorkspaceChipIdentity;
  scope?: WorkspaceScopeMode;
  /** True while workspace/scope resolution is still loading — mutes locked-state claims. */
  scopePending?: boolean;
  toggleLocked?: boolean;
  canDisableGlobal?: boolean;
  rememberedWorkspaceName?: string | null;
  onToggleGlobalScope?: () => void;
  onSelectWorkspace: (workspaceId: string) => void;
  onAddWorkspace: () => void;
  /** Runs a registry command through the one dispatch seam. */
  onRunCommand: (commandId: string) => void;
  activeOverlay: DesktopOverlay | null;
  onOverlayOpenChange: (overlay: DesktopOverlay, open: boolean) => void;
  attention: OsAttentionModel;
  /** Whether either update track currently offers an install. */
  updateAvailable: boolean;
  /** Same worktree query the switcher reads; omitted, the menu stays flat. */
  worktreesByWorkspace?: WorktreeListingByWorkspace;
  userHomeDir?: string;
  /** Focused window's scope, resolved against the live list. */
  worktreeSelection?: ActiveWorktreeSelection;
  onSelectWorktree?: (workspaceId: string, entry: WorktreeNestEntry) => void;
  onCreateWorktree?: (workspaceId: string) => void;
  onResolveMissingWorktree?: (workspaceId: string, entry: WorktreeNestEntry) => void;
  onOpenWorktreeContext?: (workspaceId: string, entry: WorktreeNestEntry) => void;
  onRemoveWorktree?: (workspaceId: string, entry: WorktreeNestEntry) => void;
  /**
   * Profile switcher, supplied by the shell. It owns its own reads, so it is
   * injected rather than constructed here — the bar stays presentational.
   */
  profileSwitcher?: React.ReactNode;
}

/**
 * The wired menubar: the CompozyOS system menu, the workspace switcher, the static
 * Session / Go / Window / Help set, the bell aggregator, the palette button, and the
 * settings cog. All actions are runtime-backed — no menu item renders without a
 * working mechanism, and none is hidden when its predicate fails (SD-007).
 */
export function DesktopMenubar({
  className,
  workspaces,
  activeWorkspace,
  chip,
  scope = activeWorkspace ? "workspace" : "global",
  scopePending = false,
  toggleLocked = false,
  canDisableGlobal = Boolean(activeWorkspace),
  rememberedWorkspaceName,
  onToggleGlobalScope,
  onSelectWorkspace,
  onAddWorkspace,
  onRunCommand,
  activeOverlay,
  onOverlayOpenChange,
  attention,
  updateAvailable,
  worktreesByWorkspace,
  userHomeDir,
  worktreeSelection,
  onSelectWorktree,
  onCreateWorktree,
  onResolveMissingWorktree,
  onOpenWorktreeContext,
  onRemoveWorktree,
  profileSwitcher,
}: DesktopMenubarProps) {
  const hydration = useDesktop(state => state.hydration);
  const actions = useMenubarActions();
  const paletteOpen = usePaletteCommand("palette.open");
  const scopeModel = desktopMenubarScopeModel({
    activeWorkspace,
    canDisableGlobal,
    chip,
    rememberedWorkspaceName,
    scope,
    scopePending,
    toggleLocked,
    worktreeSelection,
  });
  const overlay = (id: DesktopOverlay) => ({
    open: activeOverlay === id,
    onOpenChange: (open: boolean) => onOverlayOpenChange(id, open),
  });

  const focusAttentionRow = useMenubarAttentionSelection(onOverlayOpenChange);

  return (
    <OsMenuBar
      className={className}
      workspace={scopeModel.workspace}
      scopeNotice={
        scopeModel.fallback ? (
          <span
            role="status"
            data-testid="os-worktree-fallback-notice"
            className="inline-flex items-center gap-1.5 rounded-md bg-info-tint px-2 py-1 text-form-label text-info"
          >
            {scopeModel.fallbackNotice}
          </span>
        ) : null
      }
      // No workspace means no layout stream — there is nothing to be out of sync with.
      status={<OsHydrationStatus hydration={hydration} />}
      updateIndicator={
        <MenubarUpdateIndicator available={updateAvailable} onActivate={actions.openUpdates} />
      }
      notifications={attention.notificationCount}
      onCommandClick={() => onRunCommand("palette.open")}
      onSettingsClick={() => onRunCommand("settings.general")}
      commandShortcutLabel={paletteOpen?.chords[0]}
      logoMenu={trigger => (
        <CompozyMenu
          trigger={trigger}
          {...overlay("compozy-menu")}
          onAbout={() => onOverlayOpenChange("about", true)}
          onRun={onRunCommand}
        />
      )}
      scopeControl={
        <GlobalScopeToggle
          checked={scopeModel.globalOn}
          locked={scopePending || toggleLocked || (scopeModel.globalOn && !canDisableGlobal)}
          lockedReason={scopeModel.toggleLockedReason}
          onCheckedChange={() => onToggleGlobalScope?.()}
          tooltip={scopeModel.toggleTooltip}
        />
      }
      workspaceMenu={trigger => (
        <WorkspaceMenu
          trigger={trigger}
          {...overlay("workspace-menu")}
          workspaces={workspaces}
          activeWorkspaceId={scopeModel.globalOn ? undefined : activeWorkspace?.id}
          globalScopeOn={scopeModel.globalOn}
          monogram={workspaceMonogram}
          onSelectWorkspace={onSelectWorkspace}
          onRun={onRunCommand}
          onAddWorkspace={onAddWorkspace}
          worktreesByWorkspace={worktreesByWorkspace}
          userHomeDir={userHomeDir}
          selectedWorktreeId={
            scopeModel.globalOn ? null : (worktreeSelection?.selectedWorktreeId ?? null)
          }
          onSelectWorktree={onSelectWorktree}
          onCreateWorktree={onCreateWorktree}
          onResolveMissingWorktree={onResolveMissingWorktree}
          onOpenWorktreeContext={onOpenWorktreeContext}
          onRemoveWorktree={onRemoveWorktree}
        />
      )}
      menus={
        actions.menusVisible ? (
          <>
            <SessionMenu
              {...overlay("session-menu")}
              onNewAgent={actions.newAgent}
              onRun={onRunCommand}
            />
            <GoMenu {...overlay("go-menu")} onRun={onRunCommand} />
            <WindowMenu {...overlay("window-menu")} onRun={onRunCommand} />
            <HelpMenu {...overlay("help-menu")} onRun={onRunCommand} />
          </>
        ) : null
      }
      profileSwitcher={profileSwitcher}
      wrapBellTrigger={trigger => (
        <Popover
          open={activeOverlay === "bell"}
          onOpenChange={open => onOverlayOpenChange("bell", open)}
        >
          <PopoverTrigger render={trigger} />
          <PopoverContent
            align="end"
            aria-label="Attention"
            className="w-80 max-w-[calc(100vw-16px)] p-2"
            data-testid="os-bell-popover"
          >
            <AttentionBell
              sections={attention.sections}
              sessionsDisconnected={attention.attentionSessionsDisconnected}
              tasksDisconnected={attention.tasksDisconnected}
              loopRequestsDisconnected={attention.loopRequestsDisconnected}
              loading={attention.loading}
              onSelect={focusAttentionRow}
            />
          </PopoverContent>
        </Popover>
      )}
    />
  );
}
