import { useDesktopDock } from "../hooks/use-desktop-dock";
import type { ReactNode } from "react";

import { cn } from "@compozy/ui";

import type { OsAttentionBadges } from "../lib/attention-model";
import type { OsAppId } from "../lib/os-types";
import { OsDockZone } from "./os-dock";
import { OsDockAppMenu } from "./os-dock-app-menu";
import { OsDockTabBar } from "./os-dock-tab-bar";

export interface DesktopDockProps {
  onNewSession: () => void;
  badges: OsAttentionBadges;
  /** Modal overlays own keyboard and pointer interaction until they close. */
  contextMenusEnabled: boolean;
  pager: ReactNode;
  /** First run: the dock is present but asleep until setup commits. */
  dormant?: boolean;
}

/** Zone-level wake — the dock lifts back as one surface when setup finishes. */
const DORMANT = "translate-y-1.5 opacity-50 saturate-50";
const WAKE =
  "transition-[opacity,filter,transform] duration-shell-slow ease-spring motion-reduce:transition-none";

/**
 * The wired dock: floating renders the centered glass strip with proximity
 * magnification; compact renders the full-width bottom tab bar (os-v2.css
 * mobile block). Entries, activation semantics, and the magnification gates
 * live in `useDesktopDock`.
 */
export function DesktopDock({
  onNewSession,
  badges,
  contextMenusEnabled,
  pager,
  dormant = false,
}: DesktopDockProps) {
  const { entries, presentation, magnify, handleSelect } = useDesktopDock(badges, {
    onNewSession,
  });
  const dormancy = cn(WAKE, dormant && DORMANT);

  if (presentation === "compact") {
    return (
      <OsDockTabBar
        className={dormancy}
        items={entries}
        leading={pager}
        onSelect={handleSelect}
        onNewSession={onNewSession}
      />
    );
  }

  return (
    <OsDockZone
      className={dormancy}
      items={entries}
      leading={pager}
      onSelect={handleSelect}
      renderItemMenu={
        !contextMenusEnabled
          ? undefined
          : (item, children) =>
              item.id === "session" ? (
                children
              ) : (
                <OsDockAppMenu appId={item.id as OsAppId}>{children}</OsDockAppMenu>
              )
      }
      onNewSession={onNewSession}
      magnify={magnify}
    />
  );
}
