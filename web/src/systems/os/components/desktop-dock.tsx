import { useDesktopDock } from "../hooks/use-desktop-dock";
import { useTerminalDockRunning } from "../hooks/use-terminal-dock-running";
import { cn } from "@compozy/ui";

import type { OsAttentionBadges } from "../lib/attention-model";
import type { OsAppId } from "../lib/os-types";
import { OsDock } from "./os-dock";
import { OsDockAppMenu } from "./os-dock-app-menu";
import { OsRailFoot } from "./os-dock-rail-foot";
import { OsDockTabBar } from "./os-dock-tab-bar";

export interface DesktopDockProps {
  onNewSession: () => void;
  /** Global scope with no sessions: the Sessions launcher opens the workspace switcher. */
  onPickProject?: () => void;
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
  onPickProject,
  badges,
  terminalLive,
  contextMenusEnabled,
  dormant = false,
  profileSwitcher,
  onOpenSettings,
}: DesktopDockProps) {
  const { entries, presentation, commandsAvailable, handleSelect } = useDesktopDock(badges, {
    onNewSession,
    onPickProject,
    terminalLive,
  });
  const dormancy = cn(WAKE, dormant && DORMANT);

  if (presentation === "compact") {
    return (
      <OsDockTabBar
        className={cn("col-span-full row-start-3", dormancy)}
        items={entries}
        onSelect={handleSelect}
        disabled={!commandsAvailable}
        trailing={
          <OsRailFoot
            profileSwitcher={profileSwitcher}
            onOpenSettings={onOpenSettings}
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
      disabled={!commandsAvailable}
      renderItemMenu={
        !commandsAvailable || !contextMenusEnabled
          ? undefined
          : (item, children) =>
              item.id === "session" ? (
                children
              ) : (
                <OsDockAppMenu appId={item.id as OsAppId}>{children}</OsDockAppMenu>
              )
      }
      foot={<OsRailFoot profileSwitcher={profileSwitcher} onOpenSettings={onOpenSettings} />}
    />
  );
}

/**
 * Production dock: Terminal's live mark reads the catalog, not window-open.
 * Isolated dock tests keep rendering `DesktopDock` without this query.
 */
export function ShellDesktopDock(props: Omit<DesktopDockProps, "terminalLive">) {
  const terminalLive = useTerminalDockRunning();
  return <DesktopDock {...props} terminalLive={terminalLive} />;
}
