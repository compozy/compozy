import { useShallow } from "zustand/shallow";

import { dockBadgeFor } from "../components/dock-badges";
import { DockIcons, type DockIconId } from "../components/os-dock-icons";
import type { OsDockEntry } from "../components/os-dock-types";
import type { OsAttentionBadges } from "../lib/attention-model";
import { dockApps, OS_APPS } from "../lib/app-registry";
import { osWindowId, type OsAppId, type OsPresentation } from "../lib/os-types";
import { useDesktop } from "./use-desktop";
import { useOsShell } from "./use-os-shell";

export interface DesktopDockModel {
  entries: OsDockEntry[];
  presentation: OsPresentation;
  /** Floating-only magnification, gated by the appearance toggle and motion prefs. */
  magnify: boolean;
  handleSelect: (id: string) => void;
}

export interface UseDesktopDockOptions {
  sessionsOpen: boolean;
  onToggleSessions: () => void;
}

/**
 * Dock view-model: registry groups with running/minimized/badge state, the
 * open-or-minimize activation semantics (tab-bar taps never minimize —
 * os-v2.js:462), and the magnification gates the prototype applies.
 */
export function useDesktopDock(
  badges: OsAttentionBadges,
  { sessionsOpen, onToggleSessions }: UseDesktopDockOptions
): DesktopDockModel {
  const { coordinator } = useOsShell();
  const presentation = useDesktop(state => state.presentation);
  // Magnification composes every gate the prototype applies (os-v2.js): the
  // appearance toggle here, the system reduced-motion preference inside
  // `useDockMagnify`, and compact presentation via the tab-bar branch.
  const magnify = useDesktop(state => state.dockMagnify && !state.reduceMotion);
  const windowStates = useDesktop(
    useShallow(state => {
      const byApp: Record<string, "open" | "focused" | "minimized"> = {};
      for (const win of Object.values(state.windows)) {
        if (win.instanceKey !== null) continue;
        byApp[win.app] = win.minimized
          ? "minimized"
          : state.focusedId === win.id
            ? "focused"
            : "open";
      }
      return byApp;
    })
  );

  const groups = dockApps();
  const sessionApp = OS_APPS.session;
  const entries: OsDockEntry[] = [
    {
      id: sessionApp.id,
      name: "Sessions",
      icon: DockIcons.sessions,
      running: sessionsOpen,
      badge: dockBadgeFor(sessionApp, badges),
    },
  ];
  groups.forEach((group, index) => {
    // Prototype seams: groups 1+2 run together; separators split 2|3 and 3|4.
    if (index >= 2) entries.push({ id: `sep-${index}`, sep: true });
    for (const app of group) {
      const state = windowStates[app.id];
      entries.push({
        id: app.id,
        name: app.title,
        icon: DockIcons[app.id as DockIconId] ?? app.icon,
        running: state === "open" || state === "focused",
        minimized: state === "minimized",
        badge: dockBadgeFor(app, badges),
      });
    }
  });

  const handleSelect = (id: string) => {
    const appId = id as OsAppId;
    if (appId === "session") {
      onToggleSessions();
      return;
    }
    // Tab-bar semantics (compact): tap = switch to, never minimize.
    if (windowStates[appId] === "focused" && presentation === "floating") {
      void coordinator.userMinimize(osWindowId(appId));
      return;
    }
    void coordinator.userOpen({ app: appId });
  };

  return { entries, presentation, magnify, handleSelect };
}
