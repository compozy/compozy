import type * as React from "react";
import { fn } from "storybook/test";

import { OsDockZone, type OsDockEntry, type OsDockItemData } from "../os-dock";
import { DockIcons } from "../os-dock-icons";
import { OsMenuBar } from "../os-menubar";
import { OsWallpaper, type OsWallpaperKind } from "../os-wallpaper";

const DOCK_DEFS = [
  { id: "sessions", name: "Sessions", icon: DockIcons.sessions },
  { id: "dashboard", name: "Dashboard", icon: DockIcons.dashboard },
  { id: "agents", name: "Agents", icon: DockIcons.agents },
  { id: "network", name: "Network", icon: DockIcons.network },
  { id: "tasks", name: "Tasks", icon: DockIcons.tasks },
  { id: "loops", name: "Loops", icon: DockIcons.loops },
  { id: "jobs", name: "Jobs", icon: DockIcons.jobs },
  { id: "triggers", name: "Triggers", icon: DockIcons.triggers },
  { id: "sep-1", sep: true as const },
  { id: "marketplace", name: "Marketplace", icon: DockIcons.marketplace },
  { id: "bridges", name: "Bridges", icon: DockIcons.bridges },
  { id: "knowledge", name: "Knowledge", icon: DockIcons.knowledge },
  { id: "sep-2", sep: true as const },
  { id: "sandbox", name: "Sandbox", icon: DockIcons.sandbox },
  { id: "vault", name: "Vault", icon: DockIcons.vault },
] as const;

/** Runtime attention badges (OpenDesign DOCK_BADGES): sessions waiting, tasks needing you. */
const DEFAULT_BADGES: Record<string, number> = { sessions: 1, tasks: 1 };

/**
 * Build the full OpenDesign `DOCK_ORDER` strip with separators, applying
 * running/minimized/badge fixtures for the story's open-window set.
 */
export function buildDeskItems(opts?: {
  open?: string[];
  minimized?: string[];
  badges?: Record<string, number>;
}): OsDockEntry[] {
  const open = new Set(opts?.open ?? []);
  const minimized = new Set(opts?.minimized ?? []);
  const badges = opts?.badges ?? DEFAULT_BADGES;
  return DOCK_DEFS.map(def => {
    if ("sep" in def) return { id: def.id, sep: true as const };
    return {
      id: def.id,
      name: def.name,
      icon: def.icon,
      running: open.has(def.id),
      minimized: minimized.has(def.id),
      badge: badges[def.id],
    };
  });
}

/** Default resting fixture used by wallpaper/menubar shells (no windows open). */
export const DESK_ITEMS: OsDockEntry[] = buildDeskItems();

/** Flat app items only (no separators) — for badge-cap stories. */
export const DESK_APP_ITEMS: OsDockItemData[] = DESK_ITEMS.filter(
  (e): e is OsDockItemData => !("sep" in e && e.sep)
);

export interface DesktopShellProps {
  children?: React.ReactNode;
  wallpaper?: OsWallpaperKind;
  menubar?: boolean;
  dock?: boolean;
  /** Override dock entries (defaults to full DESK_ITEMS). */
  dockItems?: OsDockEntry[];
  /** Extra classes on the dock zone (first-run dormancy). */
  dockClassName?: string;
  /** Extra classes on the menubar (first-run dimming). */
  menubarClassName?: string;
  /** Menubar workspace slot; omit for the bound `agh` workspace. */
  workspace?: { name: string; monogram: string };
  /** Menubar approvals count; 0 renders no badge. */
  notifications?: number;
  /** Show the empty-desktop ⌘K hint (OpenDesign `desk-hint`). */
  deskHint?: boolean;
}

/**
 * Story-only full desktop shell: wallpaper + menubar + dock zone (strip +
 * detached New Session), matching the OpenDesign prototype frame so Visual
 * Contract rows compare the same composition.
 */
export function DesktopShell({
  children,
  wallpaper = "ember",
  menubar = true,
  dock = true,
  dockItems = DESK_ITEMS,
  dockClassName,
  menubarClassName,
  workspace = { name: "agh", monogram: "AG" },
  notifications = 2,
  deskHint = false,
}: DesktopShellProps) {
  return (
    <div className="relative flex h-screen w-full flex-col overflow-hidden bg-rail">
      {menubar ? (
        <OsMenuBar
          className={menubarClassName}
          workspace={workspace}
          notifications={notifications}
          onCommandClick={fn()}
          onSettingsClick={fn()}
        />
      ) : null}
      <div className="relative min-h-0 flex-1">
        <OsWallpaper wallpaper={wallpaper} />
        {deskHint ? (
          <p
            data-slot="os-desk-hint"
            className="pointer-events-none absolute inset-0 z-[1] flex items-center justify-center text-small-body text-muted"
          >
            <span className="inline-flex items-center gap-2">
              <kbd className="rounded-sm border border-line bg-elevated px-1.5 py-0.5 font-mono text-eyebrow text-subtle">
                ⌘K
              </kbd>
              to open anything — or pick a surface from the dock
            </span>
          </p>
        ) : null}
        {children}
        {dock ? (
          <OsDockZone
            className={dockClassName}
            items={dockItems}
            onSelect={fn()}
            onNewSession={fn()}
          />
        ) : null}
      </div>
    </div>
  );
}
