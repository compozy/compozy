import { Popover, PopoverContent, PopoverTrigger } from "@agh/ui";

import type { WorkspacePayload } from "@/systems/workspace";

import type { OsAttentionModel } from "../hooks/use-os-attention";
import type { DesktopOverlay } from "../hooks/use-desktop-overlays";
import type { OsAttentionRow } from "../lib/attention-model";
import { useMenubarActions } from "../hooks/use-menubar-actions";
import { useOsShell } from "../hooks/use-os-shell";
import { useDesktop } from "../hooks/use-desktop";
import { OsHydrationStatus } from "./os-hydration-status";
import { OsMenuBar } from "./os-menubar";
import { AttentionBell } from "./attention-bell";
import { AghMenu } from "./menubar/agh-menu";
import { GoMenu } from "./menubar/go-menu";
import { HelpMenu } from "./menubar/help-menu";
import { SessionMenu } from "./menubar/session-menu";
import { WindowMenu } from "./menubar/window-menu";
import { WorkspaceMenu } from "./menubar/workspace-menu";

export interface DesktopMenubarProps {
  /** Extra classes on the bar itself (first-run dimming). */
  className?: string;
  workspaces: WorkspacePayload[];
  activeWorkspace: WorkspacePayload | undefined;
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
}

function workspaceMonogram(name: string): string {
  return name.trim().slice(0, 2).toUpperCase() || "WS";
}

/** Nothing is bound yet — the slot says so instead of showing an empty dash. */
const UNBOUND_WORKSPACE = { name: "No workspace", monogram: "··" } as const;

/**
 * The wired menubar: the AGH system menu, the workspace switcher, the static
 * Session / Go / Window / Help set, the bell aggregator, the ⌘K chip, and the
 * settings cog. All actions are runtime-backed — no menu item renders without a
 * working mechanism, and none is hidden when its predicate fails (SD-007).
 */
export function DesktopMenubar({
  className,
  workspaces,
  activeWorkspace,
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
}: DesktopMenubarProps) {
  const { coordinator } = useOsShell();
  const hydration = useDesktop(state => state.hydration);
  const actions = useMenubarActions();
  const bound = activeWorkspace !== undefined;
  const workspaceSlot = bound
    ? { name: activeWorkspace.name, monogram: workspaceMonogram(activeWorkspace.name) }
    : UNBOUND_WORKSPACE;
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
      workspace={workspaceSlot}
      // No workspace means no layout stream — there is nothing to be out of sync with.
      status={bound ? <OsHydrationStatus hydration={hydration} /> : null}
      notifications={attention.notificationCount}
      onCommandClick={onOpenPalette}
      onSettingsClick={actions.openSettings}
      logoMenu={trigger => (
        <AghMenu
          trigger={trigger}
          {...overlay("agh-menu")}
          canOpenApps={actions.canOpenApps}
          onAbout={() => onOverlayOpenChange("about", true)}
          onSettings={actions.openSettings}
          onAppearance={actions.openAppearance}
          onLayouts={actions.openLayouts}
        />
      )}
      workspaceMenu={trigger => (
        <WorkspaceMenu
          trigger={trigger}
          {...overlay("workspace-menu")}
          workspaces={workspaces}
          activeWorkspaceId={activeWorkspace?.id}
          monogram={workspaceMonogram}
          onSelectWorkspace={onSelectWorkspace}
          onOpenWorkspaces={onOpenWorkspaces}
          onAddWorkspace={onAddWorkspace}
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
