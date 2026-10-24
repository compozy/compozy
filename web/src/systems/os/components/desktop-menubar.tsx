import { useEffect } from "react";

import type { WorktreeRemovalBatch } from "@/systems/workspace";
import { Popover, PopoverContent, PopoverTrigger } from "@compozy/ui";

import { usePaletteCommand } from "../hooks/use-palette-registry";
import type { OsAttentionModel } from "../hooks/use-os-attention";
import type { DesktopOverlay } from "../hooks/use-desktop-overlays";
import { useMenubarActions } from "../hooks/use-menubar-actions";
import { useMenubarAttentionSelection } from "../hooks/use-menubar-attention-selection";
import { useProviderUsageSurface } from "../hooks/use-provider-usage-surface";
import { useDesktop } from "../hooks/use-desktop";
import { useWindowManagerDiagnostic } from "../hooks/use-window-manager-store";
import { desktopMenubarScopeModel } from "../lib/desktop-menubar-model";
import { desktopShellBridge } from "../lib/desktop-shell-bridge";
import { primaryShortcutModifier, shortcutAriaKeys } from "../lib/window-manager-shortcuts";
import { OsHydrationStatus } from "./os-hydration-status";
import { OsMenuBar } from "./os-menubar";
import { AttentionBell } from "./attention-bell";
import {
  MenubarUpdateIndicator,
  type MenubarUpdateModel,
} from "./menubar/menubar-update-indicator";
import { CompozyMenu } from "./menubar/compozy-menu";
import { GoMenu } from "./menubar/go-menu";
import { HelpMenu } from "./menubar/help-menu";
import { MenubarUsagePopover } from "./menubar/menubar-usage-popover";
import { SessionMenu } from "./menubar/session-menu";
import { WindowMenu } from "./menubar/window-menu";
import { WorkspaceMenu } from "./menubar/workspace-menu";
import {
  type ActiveWorktreeSelection,
  type BreadthMode,
  type Focus,
  type WorkspaceChipIdentity,
  type WorkspacePayload,
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
  /** The globe: All projects, or only the focus. */
  breadthMode?: BreadthMode;
  /** The focused destination; under All projects, where the globe returns. */
  focus?: Focus;
  /** True while destination resolution is still loading — the globe waits. */
  scopePending?: boolean;
  /** Name of the focused project, for the globe's "Back to …" tooltip. */
  focusWorkspaceName?: string | null;
  onToggleAllProjects?: () => void;
  onSelectWorkspace: (workspaceId: string) => void;
  /** Switcher "User" row: focus User. */
  onFocusUser?: () => void;
  /** Switcher "All projects" row: turn All projects on. */
  onEnableAllProjects?: () => void;
  onAddWorkspace: () => void;
  /** Runs a registry command through the one dispatch seam. */
  onRunCommand: (commandId: string) => void;
  activeOverlay: DesktopOverlay | null;
  onOverlayOpenChange: (overlay: DesktopOverlay, open: boolean) => void;
  attention: OsAttentionModel;
  /** The one-click product update; `hidden` keeps the control out of the DOM. */
  update: MenubarUpdateModel;
  /** Same worktree query the switcher reads; omitted, the menu stays flat. */
  worktreesByWorkspace?: WorktreeListingByWorkspace;
  /** Working-session counts by workspace id for the menu's working dots. */
  workingSessionsByWorkspace?: Readonly<Record<string, number>>;
  userHomeDir?: string;
  /** Focused window's scope, resolved against the live list. */
  worktreeSelection?: ActiveWorktreeSelection;
  onSelectWorktree?: (workspaceId: string, entry: WorktreeNestEntry) => void;
  onCreateWorktree?: (workspaceId: string) => void;
  onResolveMissingWorktree?: (workspaceId: string, entry: WorktreeNestEntry) => void;
  onOpenWorktreeContext?: (workspaceId: string, entry: WorktreeNestEntry) => void;
  onRemoveWorktrees?: (batch: WorktreeRemovalBatch) => void;
  onRemoveWorktree?: (workspaceId: string, entry: WorktreeNestEntry) => void;
  /** Desktop pager, supplied by the shell (it owns the desktop switch wiring). */
  pager?: React.ReactNode;
  /** No project is bound, so there is no layout stream to report on. */
  layoutUnbound?: boolean;
}

/** Live window-layout status: stream health plus the latest refused-command notice. */
/** The palette chord as `aria-keyshortcuts`, mapped to this platform's primary modifier. */
function paletteKeyShortcuts(binding: string | undefined): string | undefined {
  if (!binding) return undefined;
  const platform = typeof navigator === "undefined" ? "" : navigator.platform;
  return shortcutAriaKeys(binding, primaryShortcutModifier(platform)) || undefined;
}

function MenubarLayoutStatus({ unbound }: { unbound: boolean }) {
  const hydration = useDesktop(state => state.hydration);
  const loadError = useDesktop(state => state.loadError);
  const connectionStatus = useDesktop(state => state.connectionStatus);
  const diagnostic = useWindowManagerDiagnostic();
  return (
    <OsHydrationStatus
      hydration={hydration}
      loadError={loadError}
      connectionStatus={connectionStatus}
      diagnostic={diagnostic}
      unbound={unbound}
    />
  );
}

/**
 * The wired topbar: the CompozyOS system menu (which holds Settings), the workspace
 * switcher, the static Session / Go / Window / Help set, then the tray — desktop
 * pager, All desktops, Provider usage, the bell aggregator, and the palette
 * button. All actions are runtime-backed — no menu item renders without a
 * working mechanism, and none is hidden when its predicate fails (SD-007).
 */
export function DesktopMenubar({
  className,
  workspaces,
  activeWorkspace,
  chip,
  breadthMode: breadthModeProp,
  focus: focusProp,
  scopePending = false,
  focusWorkspaceName,
  onToggleAllProjects,
  onSelectWorkspace,
  onFocusUser,
  onEnableAllProjects,
  onAddWorkspace,
  onRunCommand,
  activeOverlay,
  onOverlayOpenChange,
  attention,
  update,
  worktreesByWorkspace,
  workingSessionsByWorkspace,
  userHomeDir,
  worktreeSelection,
  onSelectWorktree,
  onCreateWorktree,
  onResolveMissingWorktree,
  onOpenWorktreeContext,
  onRemoveWorktree,
  onRemoveWorktrees,
  pager,
  layoutUnbound = false,
}: DesktopMenubarProps) {
  const breadthMode = breadthModeProp ?? (activeWorkspace ? "focused" : "all");
  const focus: Focus =
    focusProp ??
    (activeWorkspace ? { kind: "project", workspaceId: activeWorkspace.id } : { kind: "user" });
  const actions = useMenubarActions();
  const paletteOpen = usePaletteCommand("palette.open");
  const scopeModel = desktopMenubarScopeModel({
    activeWorkspace,
    breadthMode,
    chip,
    focus,
    focusWorkspaceName,
    scopePending,
    worktreeSelection,
  });
  const overlay = (id: DesktopOverlay) => ({
    open: activeOverlay === id,
    onOpenChange: (open: boolean) => onOverlayOpenChange(id, open),
  });

  const focusAttentionRow = useMenubarAttentionSelection(onOverlayOpenChange);
  const usage = useProviderUsageSurface();
  const usageVisible = usage.trayVisible;
  const usageOpen = activeOverlay === "usage";

  // The control leaves when its last target does (feature off, provider
  // turned off); its overlay slot must not outlive it.
  useEffect(() => {
    if (usageOpen && !usageVisible) onOverlayOpenChange("usage", false);
  }, [onOverlayOpenChange, usageOpen, usageVisible]);

  return (
    <OsMenuBar
      className={className}
      workspace={{ ...scopeModel.workspace, warning: scopeModel.fallbackNotice }}
      // The sentence lives on the chip (warning dot) and atop the project menu;
      // this live region only announces it.
      scopeNotice={
        scopeModel.fallback ? (
          <span role="status" data-testid="os-worktree-fallback-notice" className="sr-only">
            {scopeModel.fallbackNotice}
          </span>
        ) : null
      }
      // No workspace means no layout stream — there is nothing to be out of sync with.
      status={<MenubarLayoutStatus unbound={layoutUnbound} />}
      updateIndicator={
        <MenubarUpdateIndicator
          onOpenUpdates={actions.openUpdates}
          onUpdate={update.onUpdate}
          state={update.state}
        />
      }
      notifications={attention.notificationCount}
      onCommandClick={() => onRunCommand("palette.open")}
      onDesktopsClick={() => onOverlayOpenChange("desktops", activeOverlay !== "desktops")}
      desktopsOpen={activeOverlay === "desktops"}
      trafficLights={desktopShellBridge()?.platform === "darwin"}
      commandShortcutLabel={paletteOpen?.chords[0]}
      commandKeyShortcuts={paletteKeyShortcuts(paletteOpen?.bindings[0])}
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
          checked={scopeModel.allProjectsOn}
          locked={scopePending}
          onCheckedChange={() => onToggleAllProjects?.()}
          tooltip={scopeModel.toggleTooltip}
        />
      }
      workspaceMenu={trigger => (
        <WorkspaceMenu
          trigger={trigger}
          {...overlay("workspace-menu")}
          workspaces={workspaces}
          breadthMode={breadthMode}
          focus={focus}
          onSelectWorkspace={onSelectWorkspace}
          onFocusUser={onFocusUser}
          onEnableAllProjects={onEnableAllProjects}
          onRun={onRunCommand}
          onAddWorkspace={onAddWorkspace}
          worktreesByWorkspace={worktreesByWorkspace}
          workingSessionsByWorkspace={workingSessionsByWorkspace}
          userHomeDir={userHomeDir}
          selectedWorktreeId={
            scopeModel.allProjectsOn ? null : (worktreeSelection?.selectedWorktreeId ?? null)
          }
          onSelectWorktree={onSelectWorktree}
          onCreateWorktree={onCreateWorktree}
          onResolveMissingWorktree={onResolveMissingWorktree}
          onOpenWorktreeContext={onOpenWorktreeContext}
          onRemoveWorktree={onRemoveWorktree}
          onRemoveWorktrees={onRemoveWorktrees}
          notice={scopeModel.fallbackNotice}
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
      pager={pager}
      usageControl={
        usageVisible
          ? className => (
              <MenubarUsagePopover
                surface={usage}
                triggerClassName={className}
                {...overlay("usage")}
              />
            )
          : undefined
      }
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
              total={attention.notificationTotal}
              pending={attention.acknowledging}
              error={attention.acknowledgementError}
              onAcknowledge={attention.onAcknowledge}
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
