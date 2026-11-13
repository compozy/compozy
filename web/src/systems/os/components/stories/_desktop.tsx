import type * as React from "react";
import { cn } from "@compozy/ui";
import { fn } from "storybook/test";

import { OsDock, type OsDockItemData } from "../os-dock";
import { OsRailFoot } from "../os-dock-rail-foot";
import { OsDockTabBar } from "../os-dock-tab-bar";
import { OsMenuBar } from "../os-menubar";
import { OsWallpaper, type OsWallpaperKind } from "../os-wallpaper";
import { OsWindowSurface } from "../os-window-surface";
import { shortcutLabel } from "../../lib/window-manager-shortcuts";

const DOCK_DEFS = [
  { id: "dashboard", name: "Dashboard", icon: "dashboard" },
  { id: "sessions", name: "Sessions", icon: "sessions" },
  { id: "terminal", name: "Terminal", icon: "terminal" },
  { id: "agents", name: "Agents", icon: "agents" },
  { id: "tasks", name: "Tasks", icon: "tasks" },
  { id: "loops", name: "Loops", icon: "loops" },
  { id: "automations", name: "Automations", icon: "automations" },
  { id: "marketplace", name: "Marketplace", icon: "marketplace" },
  { id: "vault", name: "Vault", icon: "vault" },
] as const;

/** Runtime attention badges (OpenDesign DOCK_BADGES): sessions waiting, tasks needing you. */
const DEFAULT_BADGES: Record<string, number> = { sessions: 1, tasks: 1 };

/**
 * Build the full rail order, applying running/active/badge fixtures
 * for the story's open-window set. `active` names the focused window's app.
 */
export function buildDeskItems(opts?: {
  open?: string[];
  active?: string;
  badges?: Record<string, number>;
}): OsDockItemData[] {
  const open = new Set(opts?.open ?? []);
  const badges = opts?.badges ?? DEFAULT_BADGES;
  return DOCK_DEFS.map(def => ({
    id: def.id,
    name: def.name,
    icon: def.icon,
    running: open.has(def.id) || opts?.active === def.id,
    active: opts?.active === def.id,
    badge: badges[def.id],
  }));
}

/** Default resting fixture used by wallpaper/menubar shells (no windows open). */
export const DESK_ITEMS: OsDockItemData[] = buildDeskItems();

export interface DesktopShellProps {
  children?: React.ReactNode;
  wallpaper?: OsWallpaperKind;
  menubar?: boolean;
  dock?: boolean;
  /** Override dock entries (defaults to full DESK_ITEMS). */
  dockItems?: OsDockItemData[];
  /** Extra classes on the rail (first-run dormancy). */
  dockClassName?: string;
  /** Extra classes on the menubar (first-run dimming). */
  menubarClassName?: string;
  /** Menubar workspace slot; omit for the bound `compozy` workspace. */
  workspace?: { name: string };
  /** Menubar approvals count; 0 renders no badge. */
  notifications?: number;
  /** Show a resting desk: its one tab holds a New Tab window. */
  deskHint?: boolean;
  /** The desktop's tab bar, between the topbar and the desk. */
  tabBar?: React.ReactNode;
  /** Compact (<960px) presentation: the bottom tab bar replaces the rail. */
  compact?: boolean;
  /** Topbar desktop pager slot. */
  pager?: React.ReactNode;
  /** Replaces the default topbar (wired `DesktopMenubar` fixtures). */
  topbar?: React.ReactNode;
}

/**
 * Story-only full desktop shell: topbar across the full width, the rail and the
 * wallpapered desk panel inset below it — the production `DesktopShellScopedBody`
 * grid, so Visual Contract rows compare the same composition. `dock={false}` drops the
 * rail and gives the desk the full width.
 */
export function DesktopShell({
  children,
  wallpaper = "flat",
  menubar = true,
  dock = true,
  dockItems = DESK_ITEMS,
  dockClassName,
  menubarClassName,
  workspace = { name: "compozy" },
  notifications = 2,
  deskHint = false,
  compact = false,
  pager,
  topbar,
  tabBar,
}: DesktopShellProps) {
  // The desk is inset into the chrome only where chrome surrounds it.
  const hasTopbar = menubar || topbar != null;
  const hasRail = dock && !compact;
  return (
    <div className="relative grid h-screen w-full grid-cols-[auto_minmax(0,1fr)] grid-rows-[auto_auto_minmax(0,1fr)_auto] overflow-hidden bg-rail">
      {topbar ? <div className="col-span-full">{topbar}</div> : null}
      {menubar && !topbar ? (
        <OsMenuBar
          className={cn("col-span-full", menubarClassName)}
          workspace={workspace}
          notifications={notifications}
          onCommandClick={fn()}
          onDesktopsClick={fn()}
          commandShortcutLabel={shortcutLabel("meta+KeyK")}
          pager={pager}
        />
      ) : null}
      {dock && compact ? (
        <OsDockTabBar
          className={cn("col-span-full row-start-4", dockClassName)}
          items={dockItems}
          onSelect={fn()}
          trailing={<OsRailFoot onOpenSettings={fn()} tipSide="top" />}
        />
      ) : null}
      {dock && !compact ? (
        <OsDock
          className={cn("col-start-1 row-span-2 row-start-2", dockClassName)}
          items={dockItems}
          onSelect={fn()}
          foot={<OsRailFoot onOpenSettings={fn()} />}
        />
      ) : null}
      {tabBar ? (
        <div className={cn("row-start-2 min-w-0", hasRail ? "col-start-2" : "col-span-full")}>
          {tabBar}
        </div>
      ) : null}
      <div
        data-slot="os-desk"
        className={cn(
          "relative col-start-2 row-start-3 min-h-0 overflow-hidden border-line",
          hasTopbar && "border-t",
          hasRail && "border-l",
          hasTopbar && hasRail && "rounded-tl-lg"
        )}
      >
        <OsWallpaper wallpaper={wallpaper} />
        {deskHint ? <OsWindowSurface className="absolute inset-0" title="New tab" /> : null}
        {children}
      </div>
    </div>
  );
}
