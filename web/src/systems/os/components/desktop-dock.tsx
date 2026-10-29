import { useCallback } from "react";

import { useDesktopDock, type DockPreload } from "../hooks/use-desktop-dock";
import { useTerminalDockRunning } from "../hooks/use-terminal-dock-running";
import { useTerminalLauncherEntry } from "../hooks/use-terminal-launcher";
import { useShellDockPreload } from "../hooks/use-shell-dock-preload";
import { cn } from "@compozy/ui";

import type { OsAttentionBadges } from "../lib/attention-model";
import type { OsAppId } from "../lib/os-types";
import { OsDock } from "./os-dock";
import { OsDockAppMenu } from "./os-dock-app-menu";
import { OsRailFoot } from "./os-dock-rail-foot";
import { OsDockTabBar } from "./os-dock-tab-bar";
import { TerminalLauncherDialog } from "./terminal-launcher-dialog";

export interface DesktopDockProps {
  onNewSession: () => void;
  badges: OsAttentionBadges;
  /** Catalog truth: a live terminal, independent of an open window. */
  terminalLive?: boolean;
  /** Modal overlays own keyboard and pointer interaction until they close. */
  contextMenusEnabled: boolean;
  /** First run: the dock is present but asleep until setup commits. */
  dormant?: boolean;
  /** Profile switcher, supplied by the shell; the rail foot (or compact tab bar) hosts it. */
  profileSwitcher?: React.ReactNode;
  onOpenSettings: () => void;
  onPreload?: DockPreload;
  /** Opens the Terminal launcher instead of an empty Terminal window. */
  onOpenTerminalLauncher?: () => void;
}

/** Wake as one surface — the dock brightens back when setup finishes. */
const DORMANT = "opacity-50 saturate-50";
const WAKE =
  "transition-[opacity,filter] duration-shell-slow ease-spring motion-reduce:transition-none";

/**
 * The wired dock: floating renders the left rail beside the desktop; compact
 * renders the full-width bottom tab bar. Each claims its own row/column of the
 * shell grid (`DesktopShellScopedBody`) and carries the rail-foot controls.
 * Entries and activation semantics live in `useDesktopDock`.
 */
export function DesktopDock({
  onNewSession,
  badges,
  terminalLive,
  contextMenusEnabled,
  dormant = false,
  profileSwitcher,
  onOpenSettings,
  onPreload,
  onOpenTerminalLauncher,
}: DesktopDockProps) {
  const { entries, presentation, commandsAvailable, handleSelect, handleLaunch, handlePreload } =
    useDesktopDock(badges, {
      onNewSession,
      onPreload: dormant || !contextMenusEnabled ? undefined : onPreload,
      terminalLive,
      onOpenTerminalLauncher,
    });
  const dormancy = cn(WAKE, dormant && DORMANT);
  const canPreload = !!onPreload && !dormant && contextMenusEnabled;
  const preloadSettings = useCallback(() => handlePreload("settings"), [handlePreload]);

  if (presentation === "compact") {
    return (
      <OsDockTabBar
        className={cn("col-span-full row-start-3", dormancy)}
        items={entries}
        onSelect={handleSelect}
        onPreload={canPreload ? handlePreload : undefined}
        disabled={!commandsAvailable}
        trailing={
          <OsRailFoot
            profileSwitcher={profileSwitcher}
            onOpenSettings={onOpenSettings}
            onPreloadSettings={canPreload ? preloadSettings : undefined}
            tipSide="top"
          />
        }
      />
    );
  }

  return (
    <OsDock
      className={cn("col-start-1 row-start-2", dormancy)}
      items={entries}
      onSelect={handleSelect}
      onPreload={canPreload ? handlePreload : undefined}
      disabled={!commandsAvailable}
      renderItemMenu={
        !commandsAvailable || !contextMenusEnabled
          ? undefined
          : (item, children) => (
              <OsDockAppMenu
                appId={item.id as OsAppId}
                onLaunch={placement => handleLaunch(item.id, placement)}
              >
                {children}
              </OsDockAppMenu>
            )
      }
      foot={
        <OsRailFoot
          profileSwitcher={profileSwitcher}
          onOpenSettings={onOpenSettings}
          onPreloadSettings={canPreload ? preloadSettings : undefined}
        />
      }
    />
  );
}

/**
 * Production dock: Terminal's live mark reads the catalog, not window-open,
 * and under All projects the Terminal item opens the launcher dialog.
 * Isolated dock tests keep rendering `DesktopDock` without these reads.
 */
export function ShellDesktopDock(
  props: Omit<DesktopDockProps, "terminalLive" | "onPreload" | "onOpenTerminalLauncher">
) {
  const terminalLive = useTerminalDockRunning();
  const launcher = useTerminalLauncherEntry();
  const onPreload = useShellDockPreload();
  return (
    <>
      <DesktopDock
        {...props}
        onOpenTerminalLauncher={launcher.onOpen}
        onPreload={onPreload}
        terminalLive={terminalLive}
      />
      <TerminalLauncherDialog onOpenChange={launcher.setOpen} open={launcher.open} />
    </>
  );
}
