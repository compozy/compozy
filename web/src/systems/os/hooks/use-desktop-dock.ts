import { shallowEqual } from "@xstate/store";

import type { OsAttentionBadges } from "../lib/attention-model";
import { dockAppDescriptors, OS_APP_DESCRIPTORS } from "../lib/app-catalog";
import { notifyUser } from "@/lib/user-feedback";
import { dockBadgeFor, dockIconForApp, type OsDockItemData } from "../lib/os-dock-model";
import { windowManagerCommandsAvailable } from "../lib/window-manager-command-availability";
import { activationTarget, appRunState, type OsAppRunState } from "../lib/window-instance-lookup";
import type { OsAppId, OsPresentation } from "../lib/os-types";
import { useAttentionJump } from "./use-attention-jump";
import { useDesktop } from "./use-desktop";
import { useOsShell } from "./use-os-shell";
import { useSessionLaunchCatalog } from "./use-session-launch-catalog";

export interface DesktopDockModel {
  entries: OsDockItemData[];
  presentation: OsPresentation;
  commandsAvailable: boolean;
  handleSelect: (id: string) => void;
}

export interface UseDesktopDockOptions {
  onNewSession: () => void;
  /**
   * Catalog truth for the Terminal launcher. When omitted, Terminal follows
   * the same open-window rule as every other app (isolated dock tests).
   */
  terminalLive?: boolean;
}

/**
 * Dock view-model: registry order with running/active/minimized/badge state
 * and the open-or-minimize activation semantics (tab-bar taps never minimize —
 * os-v2.js:462).
 */
export function useDesktopDock(
  badges: OsAttentionBadges,
  { onNewSession, terminalLive }: UseDesktopDockOptions
): DesktopDockModel {
  const { manager, coordinator } = useOsShell();
  const launchCatalog = useSessionLaunchCatalog();
  const jumpToSession = useAttentionJump();
  const presentation = useDesktop(state => state.presentation);
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

  const sessionApp = OS_APP_DESCRIPTORS.session;
  const sessionState = windowStates[sessionApp.id];
  // The rail keeps catalog order without group seams (shell-rail v2).
  const entries: OsDockItemData[] = [
    {
      id: sessionApp.id,
      name: "Sessions",
      icon: dockIconForApp(sessionApp),
      running: sessionState === "open" || sessionState === "focused",
      active: sessionState === "focused",
      minimized: sessionState === "minimized",
      badge: dockBadgeFor(sessionApp, badges),
    },
    ...dockAppDescriptors()
      .flat()
      .map(app => {
        const state = windowStates[app.id];
        return {
          id: app.id,
          name: app.title,
          icon: dockIconForApp(app),
          running:
            app.id === "terminal" && terminalLive !== undefined
              ? terminalLive
              : state === "open" || state === "focused",
          active: state === "focused",
          minimized: state === "minimized",
          badge: dockBadgeFor(app, badges),
        };
      }),
  ];

  const handleSelect = (id: string) => {
    if (!commandsAvailable) return;
    const appId = id as OsAppId;
    const state = manager.getState();
    if (appId === "session") {
      if (!launchCatalog.ready) return;
      // A catalog wake may still be coalesced when another client creates a
      // session. Resolve this explicit action before deciding to create one.
      void launchCatalog
        .resolveLatest()
        .then(latest => {
          if (latest === null) {
            onNewSession();
            return;
          }
          const workspaceId = latest.workspace_id?.trim() || launchCatalog.workspaceId;
          if (!workspaceId) return;
          jumpToSession({
            sessionId: latest.id,
            agentName: latest.agent_name,
            workspaceId,
          });
        })
        .catch(() => notifyUser({ message: "Couldn't load sessions. Try again.", tone: "error" }));
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

  return { entries, presentation, commandsAvailable, handleSelect };
}
