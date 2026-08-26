import { shallowEqual } from "@xstate/store";

import type { OsAttentionBadges } from "../lib/attention-model";
import { dockAppDescriptors, OS_APP_DESCRIPTORS } from "../lib/app-catalog";
import { dockBadgeFor, dockIconForApp, type OsDockEntry } from "../lib/os-dock-model";
import { windowManagerCommandsAvailable } from "../lib/window-manager-command-availability";
import {
  activationTarget,
  appRunState,
  mruWindowInstance,
  windowInstancesFor,
  type OsAppRunState,
} from "../lib/window-instance-lookup";
import type { OsAppId, OsPresentation } from "../lib/os-types";
import { useDesktop } from "./use-desktop";
import { useOsShell } from "./use-os-shell";

export interface DesktopDockModel {
  entries: OsDockEntry[];
  presentation: OsPresentation;
  /** Floating-only magnification, gated by the appearance toggle and motion prefs. */
  magnify: boolean;
  commandsAvailable: boolean;
  handleSelect: (id: string) => void;
}

export interface UseDesktopDockOptions {
  onNewSession: () => void;
}

/**
 * Dock view-model: registry groups with running/minimized/badge state, the
 * open-or-minimize activation semantics (tab-bar taps never minimize —
 * os-v2.js:462), and the magnification gates the prototype applies.
 */
export function useDesktopDock(
  badges: OsAttentionBadges,
  { onNewSession }: UseDesktopDockOptions
): DesktopDockModel {
  const { manager, coordinator } = useOsShell();
  const presentation = useDesktop(state => state.presentation);
  // Magnification composes every gate the prototype applies (os-v2.js): the
  // appearance toggle here, the system reduced-motion preference inside
  // `useDockMagnify`, and compact presentation via the tab-bar branch.
  const magnify = useDesktop(state => state.dockMagnify && !state.reduceMotion);
  const commandsAvailable = useDesktop(windowManagerCommandsAvailable);
  // Dock state aggregates every instance of an app (ADR-010 §3): the icon
  // lights while any window of that app is live, whatever its instance key.
  const windowStates = useDesktop(state => {
    const byApp: Record<string, OsAppRunState> = {};
    for (const app of Object.keys(OS_APP_DESCRIPTORS) as OsAppId[]) {
      const runState = appRunState(state.windows, state.focusedId, app);
      if (runState !== "closed") byApp[app] = runState;
    }
    return byApp;
  }, shallowEqual);

  const groups = dockAppDescriptors();
  const sessionApp = OS_APP_DESCRIPTORS.session;
  const entries: OsDockEntry[] = [
    {
      id: sessionApp.id,
      name: "Sessions",
      icon: dockIconForApp(sessionApp),
      running: windowStates[sessionApp.id] === "open" || windowStates[sessionApp.id] === "focused",
      minimized: windowStates[sessionApp.id] === "minimized",
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
        icon: dockIconForApp(app),
        running: state === "open" || state === "focused",
        minimized: state === "minimized",
        badge: dockBadgeFor(app, badges),
      });
    }
  });

  const handleSelect = (id: string) => {
    if (!commandsAvailable) return;
    const appId = id as OsAppId;
    const state = manager.getState();
    if (appId === "session") {
      const sessionWindows = windowInstancesFor(state.windows, { app: "session" });
      if (sessionWindows.length === 0) {
        onNewSession();
        return;
      }
      const target = mruWindowInstance(state.windows, state.client?.focusOrder ?? [], {
        app: "session",
      });
      if (target !== null) void coordinator.userActivateWindow(target.id);
      return;
    }
    // Repeat activation cycles the app's instances (ADR-002); minimizing the
    // focused one only makes sense when it is the app's sole instance.
    const target = activationTarget(
      state.windows,
      state.client?.focusOrder ?? [],
      state.focusedId,
      {
        app: appId,
      }
    );
    if (target === null) {
      void coordinator.userOpen({ app: appId });
      return;
    }
    // Tab-bar semantics (compact): tap = switch to, never minimize.
    if (target.id === state.focusedId && presentation === "floating") {
      void coordinator.userMinimize(target.id);
      return;
    }
    void coordinator.userActivateWindow(target.id);
  };

  return { entries, presentation, magnify, commandsAvailable, handleSelect };
}
