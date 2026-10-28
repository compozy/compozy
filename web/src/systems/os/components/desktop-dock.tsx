import { useCallback } from "react";
import { useQueryClient } from "@tanstack/react-query";

import { useDesktopDock, type DockPreload } from "../hooks/use-desktop-dock";
import { useTerminalDockRunning } from "../hooks/use-terminal-dock-running";
import { cn } from "@compozy/ui";
import { useActiveWorkspace } from "@/systems/workspace";
import { profileViewKey, readProfileView, useProfileReadScope } from "@/systems/profiles";
import { preloadSessionEntry } from "@/systems/session";

import type { OsAttentionBadges } from "../lib/attention-model";
import type { OsAppId } from "../lib/os-types";
import { getOsApp, preloadOsAppCode } from "../lib/app-registry";
import { OsDock } from "./os-dock";
import { OsDockAppMenu } from "./os-dock-app-menu";
import { OsRailFoot } from "./os-dock-rail-foot";
import { OsDockTabBar } from "./os-dock-tab-bar";

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
}: DesktopDockProps) {
  const { entries, presentation, commandsAvailable, handleSelect, handleLaunch, handlePreload } =
    useDesktopDock(badges, {
      onNewSession,
      onPreload: dormant || !contextMenusEnabled ? undefined : onPreload,
      terminalLive,
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
 * Production dock: Terminal's live mark reads the catalog, not window-open.
 * Isolated dock tests keep rendering `DesktopDock` without this query.
 */
export function ShellDesktopDock(props: Omit<DesktopDockProps, "terminalLive" | "onPreload">) {
  const terminalLive = useTerminalDockRunning();
  const queryClient = useQueryClient();
  const { runtimeWorkspaceId, pending } = useActiveWorkspace();
  const { key: profileKey } = useProfileReadScope();
  const onPreload = useCallback<DockPreload>(
    (appId, session) => {
      if (pending || profileKey !== profileViewKey(readProfileView(queryClient))) return;
      const reads: Promise<unknown>[] = [preloadOsAppCode(appId)];
      if (appId === "session" && session !== null) {
        reads.push(preloadSessionEntry(queryClient, session));
      } else {
        const preload = getOsApp(appId).preload;
        if (preload) reads.push(preload(queryClient, { workspaceId: runtimeWorkspaceId }));
      }
      return Promise.allSettled(reads);
    },
    [pending, profileKey, queryClient, runtimeWorkspaceId]
  );
  return <DesktopDock {...props} onPreload={onPreload} terminalLive={terminalLive} />;
}
