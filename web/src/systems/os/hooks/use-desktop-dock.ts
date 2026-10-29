import { shallowEqual } from "@xstate/store";
import { useCallback } from "react";

import type { OsAttentionBadges } from "../lib/attention-model";
import { dockAppDescriptors, OS_APP_DESCRIPTORS } from "../lib/app-catalog";
import { notifyUser } from "@/lib/user-feedback";
import {
  launchInPlacement,
  launchPlacementForModifiers,
  type LaunchModifiers,
  type LaunchPlacement,
} from "../lib/launch-placement";
import { dockBadgeFor, dockIconForApp, type OsDockItemData } from "../lib/os-dock-model";
import { windowManagerCommandsAvailable } from "../lib/window-manager-command-availability";
import { activationTarget, appRunState, type OsAppRunState } from "../lib/window-instance-lookup";
import { pickLastCreatedSession } from "../lib/last-created-session";
import type { OsAppId, OsPresentation } from "../lib/os-types";
import type { SessionPayload } from "@/systems/session";
import { useAttentionJump } from "./use-attention-jump";
import { useDesktop } from "./use-desktop";
import { useOsShell } from "./use-os-shell";
import { useSessionLaunchCatalog } from "./use-session-launch-catalog";

export interface DesktopDockModel {
  entries: OsDockItemData[];
  presentation: OsPresentation;
  commandsAvailable: boolean;
  /** Plain activation is focus-first; ⌥ / ⇧ launch a new instance (split / new desktop). */
  handleSelect: (id: string, modifiers?: LaunchModifiers) => void;
  /** An explicit destination from the launcher menu or a modifier click. */
  handleLaunch: (id: string, placement: LaunchPlacement) => void;
  handlePreload: (id: string) => void | Promise<unknown>;
}

export type DockPreload = (app: OsAppId, session: SessionPayload | null) => void | Promise<unknown>;

export interface UseDesktopDockOptions {
  onNewSession: () => void;
  onPreload?: DockPreload;
  /**
   * Catalog truth for the Terminal launcher. When omitted, Terminal follows
   * the same open-window rule as every other app (isolated dock tests).
   */
  terminalLive?: boolean;
}

/**
 * Dock view-model: registry order with running/active/minimized/badge state
 * and the open-or-minimize activation semantics (tab-bar taps never minimize —
 * os-v2.js:462). A plain launch that has to open follows the daemon's
 * new-window policy (a tab in the focused window by default).
 */
export function useDesktopDock(
  badges: OsAttentionBadges,
  { onNewSession, onPreload, terminalLive }: UseDesktopDockOptions
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
  // Intent timers retain this callback: only a changed destination or scope should cancel them.
  const handlePreload = useCallback(
    (id: string) => {
      if (!commandsAvailable || !onPreload || !(id in OS_APP_DESCRIPTORS)) return;
      const appId = id as OsAppId;
      const state = manager.getState();
      if (!windowManagerCommandsAvailable(state)) return;
      // Existing windows may be drilled into a detail, so do not warm an unrelated index.
      if (appId !== "session" && appRunState(state.windows, state.focusedId, appId) !== "closed") {
        return;
      }
      return onPreload(
        appId,
        appId === "session" && launchCatalog.ready
          ? pickLastCreatedSession(launchCatalog.sessions)
          : null
      );
    },
    [commandsAvailable, launchCatalog.ready, launchCatalog.sessions, manager, onPreload]
  );
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

  // Sessions launches its picker window: the latest session already has its
  // one home (plain click jumps there), so a new placement starts from the list.
  const handleLaunch = (id: string, placement: LaunchPlacement) => {
    if (!commandsAvailable) return;
    if (id === sessionApp.id && !launchCatalog.ready) return;
    void launchInPlacement({ manager, coordinator }, { app: id as OsAppId }, placement);
  };

  const handleSelect = (id: string, modifiers?: LaunchModifiers) => {
    if (!commandsAvailable) return;
    const placement = launchPlacementForModifiers(modifiers);
    if (placement !== null) {
      handleLaunch(id, placement);
      return;
    }
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
          // A User session omits `workspace_id`; the jump binds it to User.
          jumpToSession({
            sessionId: latest.id,
            agentName: latest.agent_name,
            workspaceId: latest.workspace_id?.trim() ?? "",
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
    // Tab-bar semantics (compact): tap = switch to, never minimize. A minimized
    // target is always restored, even while the client frame still names it
    // as focused (the topology can report the minimize first).
    if (target.id === state.focusedId && !target.minimized && presentation === "floating") {
      void coordinator.userMinimize(target.id);
      return;
    }
    void coordinator.userActivateWindow(target.id);
  };

  return { entries, presentation, commandsAvailable, handleSelect, handleLaunch, handlePreload };
}
