import type * as React from "react";
import { cn, Kbd } from "@compozy/ui";
import { fn } from "storybook/test";

import { OsDock, type OsDockItemData } from "../os-dock";
import { OsDockTabBar } from "../os-dock-tab-bar";
import { OsMenuBar } from "../os-menubar";
import { OsWallpaper, type OsWallpaperKind } from "../os-wallpaper";
import { shortcutLabel } from "../../lib/window-manager-shortcuts";

const DOCK_DEFS = [
  { id: "sessions", name: "Sessions", icon: "sessions" },
  { id: "dashboard", name: "Dashboard", icon: "dashboard" },
  { id: "terminal", name: "Terminal", icon: "terminal" },
  { id: "agents", name: "Agents", icon: "agents" },
  { id: "tasks", name: "Tasks", icon: "tasks" },
  { id: "loops", name: "Loops", icon: "loops" },
  { id: "jobs", name: "Jobs", icon: "jobs" },
  { id: "triggers", name: "Triggers", icon: "triggers" },
  { id: "marketplace", name: "Marketplace", icon: "marketplace" },
  { id: "knowledge", name: "Knowledge", icon: "knowledge" },
  { id: "vault", name: "Vault", icon: "vault" },
] as const;

/** Runtime attention badges (OpenDesign DOCK_BADGES): sessions waiting, tasks needing you. */
const DEFAULT_BADGES: Record<string, number> = { sessions: 1, tasks: 1 };

/**
 * Build the full rail order, applying running/active/minimized/badge fixtures
 * for the story's open-window set. `active` names the focused window's app.
 */
export function buildDeskItems(opts?: {
  open?: string[];
  active?: string;
  minimized?: string[];
  badges?: Record<string, number>;
}): OsDockItemData[] {
  const open = new Set(opts?.open ?? []);
  const minimized = new Set(opts?.minimized ?? []);
  const badges = opts?.badges ?? DEFAULT_BADGES;
  return DOCK_DEFS.map(def => ({
    id: def.id,
    name: def.name,
    icon: def.icon,
    running: open.has(def.id) || opts?.active === def.id,
    active: opts?.active === def.id,
    minimized: minimized.has(def.id),
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
  workspace?: { name: string; monogram: string };
  /** Menubar approvals count; 0 renders no badge. */
  notifications?: number;
  /** Show the empty-desktop ⌘K hint (OpenDesign `desk-hint`). */
  deskHint?: boolean;
  /** Compact (<960px) presentation: the bottom tab bar replaces the rail. */
  compact?: boolean;
  /** Topbar desktop pager slot. */
  pager?: React.ReactNode;
}

/**
 * Story-only full desktop shell: topbar across the full width, the rail and the
 * wallpapered desk below it — the production `DesktopShellScopedBody` grid, so
 * Visual Contract rows compare the same composition. `dock={false}` drops the
 * rail and gives the desk the full width.
 */
export function DesktopShell({
  children,
  wallpaper = "ember",
  menubar = true,
  dock = true,
  dockItems = DESK_ITEMS,
  dockClassName,
  menubarClassName,
  workspace = { name: "compozy", monogram: "CO" },
  notifications = 2,
  deskHint = false,
  compact = false,
  pager,
}: DesktopShellProps) {
  return (
    <div className="relative grid h-screen w-full grid-cols-[auto_minmax(0,1fr)] grid-rows-[auto_minmax(0,1fr)_auto] overflow-hidden bg-rail">
      {menubar ? (
        <OsMenuBar
          className={cn("col-span-full", menubarClassName)}
          workspace={workspace}
          notifications={notifications}
          onCommandClick={fn()}
          onSettingsClick={fn()}
          commandShortcutLabel={shortcutLabel("meta+KeyK")}
          pager={pager}
        />
      ) : null}
      {dock && compact ? (
        <OsDockTabBar
          className={cn("col-span-full row-start-3", dockClassName)}
          items={dockItems}
          onSelect={fn()}
          onNewSession={fn()}
        />
      ) : null}
      {dock && !compact ? (
        <OsDock
          className={cn("col-start-1 row-start-2", dockClassName)}
          items={dockItems}
          onSelect={fn()}
        />
      ) : null}
      <div className="relative col-start-2 row-start-2 min-h-0">
        <OsWallpaper wallpaper={wallpaper} />
        {deskHint ? (
          <p
            data-slot="os-desk-hint"
            className="pointer-events-none absolute inset-0 z-[1] flex items-center justify-center text-small-body text-muted"
          >
            <span className="inline-flex items-center gap-2">
              <Kbd>⌘K</Kbd>
              to open anything — or pick a surface from the dock
            </span>
          </p>
        ) : null}
        {children}
      </div>
    </div>
  );
}
