import { shallowEqual } from "@xstate/store";

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
import type { OsAppId, OsPresentation } from "../lib/os-types";
import { useAttentionJump } from "./use-attention-jump";
import { useDesktop } from "./use-desktop";
import { useOsShell } from "./use-os-shell";
import { useSessionLaunchCatalog } from "./use-session-launch-catalog";
import { GLOBAL_SCOPE_COPY } from "@/systems/workspace";

export interface DesktopDockModel {
  entries: OsDockItemData[];
  presentation: OsPresentation;
  commandsAvailable: boolean;
  /** Plain activation is focus-first; ⌥ / ⇧ launch a new instance (split / new desktop). */
  handleSelect: (id: string, modifiers?: LaunchModifiers) => void;
  /** An explicit destination from the launcher menu or a modifier click. */
  handleLaunch: (id: string, placement: LaunchPlacement) => void;
}

export interface UseDesktopDockOptions {
  onNewSession: () => void;
  /**
   * Global scope with no sessions: there is no project to start one in, so the
   * Sessions launcher hands the choice to the workspace switcher instead.
   */
  onPickProject?: () => void;
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
  { onNewSession, onPickProject, terminalLive }: UseDesktopDockOptions
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
  // Global with no sessions: nothing to jump to and no project to start one in.
  const needsProject =
    onPickProject !== undefined &&
    launchCatalog.ready &&
    launchCatalog.workspaceId === null &&
    launchCatalog.sessions.length === 0;
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
      hint: needsProject ? GLOBAL_SCOPE_COPY.newSessionNeedsProject : undefined,
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
    if (id === sessionApp.id) {
      if (!launchCatalog.ready) return;
      if (needsProject) {
        onPickProject();
        return;
      }
    }
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
      if (needsProject) {
        onPickProject();
        return;
      }
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
    // Tab-bar semantics (compact): tap = switch to, never minimize. A minimized
    // target is always restored, even while the client frame still names it
    // as focused (the topology can report the minimize first).
    if (target.id === state.focusedId && !target.minimized && presentation === "floating") {
      void coordinator.userMinimize(target.id);
      return;
    }
    void coordinator.userActivateWindow(target.id);
  };

  return { entries, presentation, commandsAvailable, handleSelect, handleLaunch };
}
