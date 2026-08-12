import { Popover, PopoverContent, PopoverTrigger } from "@compozy/ui";

import type { OsAttentionModel } from "../hooks/use-os-attention";
import type { DesktopOverlay } from "../hooks/use-desktop-overlays";
import type { OsAttentionRow } from "../lib/attention-model";
import { useMenubarActions } from "../hooks/use-menubar-actions";
import { useOsShell } from "../hooks/use-os-shell";
import { useDesktop } from "../hooks/use-desktop";
import { OsHydrationStatus } from "./os-hydration-status";
import { OsMenuBar } from "./os-menubar";
import { AttentionBell } from "./attention-bell";
import { CompozyMenu } from "./menubar/compozy-menu";
import { GoMenu } from "./menubar/go-menu";
import { HelpMenu } from "./menubar/help-menu";
import { SessionMenu } from "./menubar/session-menu";
import { WindowMenu } from "./menubar/window-menu";
import { WorkspaceMenu } from "./menubar/workspace-menu";
import {
  GLOBAL_SCOPE_COPY,
  globalScopeTooltipOn,
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
  deletionNotice?: string | null;
  rememberedWorkspaceName?: string | null;
  onToggleGlobalScope?: () => void;
  onSelectWorkspace: (workspaceId: string) => void;
  onAddWorkspace: () => void;
  onNewSession: () => void;
  onOpenPalette: () => void;
  onOpenDesktops: () => void;
  onOpenWorkspaces: () => void;
  onToggleSessions: () => void;
  activeOverlay: DesktopOverlay | null;
  onOverlayOpenChange: (overlay: DesktopOverlay, open: boolean) => void;
  attention: OsAttentionModel;
  /** Same worktree query the switcher reads; omitted, the menu stays flat. */
  worktreesByWorkspace?: WorktreeListingByWorkspace;
  userHomeDir?: string;
  /** Focused window's scope, resolved against the live list. */
  worktreeSelection?: ActiveWorktreeSelection;
  onSelectWorktree?: (workspaceId: string, entry: WorktreeNestEntry) => void;
  onCreateWorktree?: (workspaceId: string) => void;
}

/**
 * The wired menubar: the CompozyOS system menu, the workspace switcher, the static
 * Session / Go / Window / Help set, the bell aggregator, the ⌘K chip, and the
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
  deletionNotice = null,
  rememberedWorkspaceName,
  onToggleGlobalScope,
  onSelectWorkspace,
  onAddWorkspace,
  onNewSession,
  onOpenPalette,
  onOpenDesktops,
  onOpenWorkspaces,
  onToggleSessions,
  activeOverlay,
  onOverlayOpenChange,
  attention,
  worktreesByWorkspace,
  userHomeDir,
  worktreeSelection,
  onSelectWorktree,
  onCreateWorktree,
}: DesktopMenubarProps) {
  const { coordinator } = useOsShell();
  const hydration = useDesktop(state => state.hydration);
  const actions = useMenubarActions();
  const globalOn = scope === "global";
  if (rememberedWorkspaceName === undefined) {
    rememberedWorkspaceName = activeWorkspace ? activeWorkspace.name : null;
  }
  const workspaceSlot =
    chip ??
    (activeWorkspace
      ? { name: activeWorkspace.name, monogram: workspaceMonogram(activeWorkspace.name) }
      : { name: GLOBAL_SCOPE_COPY.chipLabel, monogram: GLOBAL_SCOPE_COPY.chipMonogram });
  // While resolution is pending nothing is claimable — neutral tooltip, no reason.
  const toggleTooltip = scopePending
    ? GLOBAL_SCOPE_COPY.tooltipOff
    : toggleLocked
      ? GLOBAL_SCOPE_COPY.tooltipLocked
      : globalOn
        ? rememberedWorkspaceName
          ? globalScopeTooltipOn(rememberedWorkspaceName)
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
  const fallbackNotice =
    fallback?.reason === "missing"
      ? `${fallback.name ?? "The selected worktree"} is missing — new work runs at the workspace root`
      : `${fallback?.name ?? "The selected worktree"} is unavailable — new work runs at the workspace root`;
  const overlay = (id: DesktopOverlay) => ({
    open: activeOverlay === id,
    onOpenChange: (open: boolean) => onOverlayOpenChange(id, open),
  });

  const focusAttentionRow = (row: OsAttentionRow) => {
    onOverlayOpenChange("bell", false);
    if (row.kind === "session") {
      void coordinator.userOpen({
        app: "session",
        instanceKey: row.id,
        route: {
          pathname: `/agents/${encodeURIComponent(row.agentName)}/sessions/${encodeURIComponent(row.id)}`,
          search: {},
        },
      });
      return;
    }
    void coordinator.userOpen({
      app: "tasks",
      route: { pathname: `/tasks/${encodeURIComponent(row.id)}`, search: {} },
    });
  };

  return (
    <OsMenuBar
      className={className}
      workspace={{
        ...workspaceSlot,
        // Global cannot bind a worktree; only a ready project worktree reaches the chip.
        worktree: globalOn ? null : (worktreeSelection?.activeWorktree?.name ?? null),
      }}
      scopeNotice={
        fallback ? (
          <span
            role="status"
            data-testid="os-worktree-fallback-notice"
            className="inline-flex items-center gap-1.5 rounded-md bg-info-tint px-2 py-1 text-form-label text-info"
          >
            {fallbackNotice}
          </span>
        ) : null
      }
      // No workspace means no layout stream — there is nothing to be out of sync with.
      status={<OsHydrationStatus hydration={hydration} />}
      notifications={attention.notificationCount}
      onCommandClick={onOpenPalette}
      onSettingsClick={actions.openSettings}
      logoMenu={trigger => (
        <CompozyMenu
          trigger={trigger}
          {...overlay("compozy-menu")}
          canOpenApps={actions.canOpenApps}
          onAbout={() => onOverlayOpenChange("about", true)}
          onSettings={actions.openSettings}
          onAppearance={actions.openAppearance}
          onLayouts={actions.openLayouts}
        />
      )}
      scopeControl={
        <GlobalScopeToggle
          checked={globalOn}
          locked={scopePending || toggleLocked || (globalOn && !canDisableGlobal)}
          lockedReason={toggleLockedReason}
          onCheckedChange={() => onToggleGlobalScope?.()}
          tooltip={toggleTooltip}
        />
      }
      workspaceMenu={trigger => (
        <WorkspaceMenu
          trigger={trigger}
          {...overlay("workspace-menu")}
          workspaces={workspaces}
          activeWorkspaceId={globalOn ? undefined : activeWorkspace?.id}
          globalScopeOn={globalOn}
          deletionNotice={deletionNotice}
          monogram={workspaceMonogram}
          onSelectWorkspace={onSelectWorkspace}
          onOpenWorkspaces={onOpenWorkspaces}
          onAddWorkspace={onAddWorkspace}
          worktreesByWorkspace={worktreesByWorkspace}
          userHomeDir={userHomeDir}
          selectedWorktreeId={globalOn ? null : (worktreeSelection?.selectedWorktreeId ?? null)}
          onSelectWorktree={onSelectWorktree}
          onCreateWorktree={onCreateWorktree}
        />
      )}
      menus={
        actions.menusVisible ? (
          <>
            <SessionMenu
              {...overlay("session-menu")}
              onNewSession={onNewSession}
              onNewAgent={actions.newAgent}
              onOpenSessions={onToggleSessions}
            />
            <GoMenu
              {...overlay("go-menu")}
              canOpenApps={actions.canOpenApps}
              onOpenPalette={onOpenPalette}
              onOpenApp={actions.openApp}
              onOpenWorkspaces={onOpenWorkspaces}
            />
            <WindowMenu
              {...overlay("window-menu")}
              commands={actions.windowCommands}
              onOpenDesktops={onOpenDesktops}
            />
            <HelpMenu
              {...overlay("help-menu")}
              canOpenApps={actions.canOpenApps}
              onOpenShortcuts={() => onOverlayOpenChange("shortcuts", true)}
              onOpenSupport={actions.openSupport}
            />
          </>
        ) : null
      }
      wrapBellTrigger={trigger => (
        <Popover
          open={activeOverlay === "bell"}
          onOpenChange={open => onOverlayOpenChange("bell", open)}
        >
          <PopoverTrigger render={trigger} />
          <PopoverContent
            align="end"
            className="w-80 max-w-[calc(100vw-16px)] p-2"
            data-testid="os-bell-popover"
          >
            <AttentionBell
              rows={attention.rows}
              sessionsDisconnected={attention.sessionsDisconnected}
              tasksDisconnected={attention.tasksDisconnected}
              loading={attention.loading}
              onSelect={focusAttentionRow}
            />
          </PopoverContent>
        </Popover>
      )}
    />
  );
}
