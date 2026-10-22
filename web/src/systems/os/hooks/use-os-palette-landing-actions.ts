import { notifyUser } from "@/lib/user-feedback";
import { consumeChooseSessionTerminalQuote } from "@/systems/session";
import {
  DESTINATION_COPY,
  ownerDataScope,
  sameDataScope,
  type DataScope,
  type WorkspacePayload,
} from "@/systems/workspace";

import { workspaceSwitchFeedback } from "../lib/cmd-palette-feedback";
import { boundRoute } from "../lib/open-target";
import type { OsAppId, OsWindowRoute } from "../lib/os-types";
import { windowManagerStore } from "../stores/window-manager-store";
import { useAttentionJump } from "./use-attention-jump";
import type {
  OsPaletteEntities,
  OsPaletteSessionResult,
  OsPaletteWorktreeResult,
} from "./use-os-palette-entities";
import { useOsShell } from "./use-os-shell";

interface UseOsPaletteLandingActionsOptions {
  close: () => void;
  destinationWindowId: string | null;
  entities: OsPaletteEntities;
  registeredWorkspaces: readonly WorkspacePayload[];
  /** The menubar destination; a session elsewhere switches it, except under All projects. */
  dataScope: DataScope | null;
}

export function useOsPaletteLandingActions({
  close,
  destinationWindowId,
  entities,
  registeredWorkspaces,
  dataScope,
}: UseOsPaletteLandingActionsOptions) {
  const { manager, coordinator } = useOsShell();
  const jumpToSession = useAttentionJump();
  const pickDestination = (target: {
    app: OsAppId;
    instanceKey?: string;
    route?: OsWindowRoute;
  }) => {
    if (destinationWindowId === null) return;
    windowManagerStore.trigger.paletteIntentCleared();
    void coordinator
      .userOpen({ ...target, stackTargetWindowId: destinationWindowId })
      .then(openedId => {
        if (openedId !== null) void manager.closeWindow(destinationWindowId);
      });
  };
  const landSession = (session: OsPaletteSessionResult) => {
    if (session.busy) return;
    consumeChooseSessionTerminalQuote(session.sessionId);
    const binding = ownerDataScope(session.workspaceId);
    // Under All projects the session opens in place; only a focused destination switches.
    if (dataScope !== null && dataScope.kind !== "all" && !sameDataScope(dataScope, binding)) {
      const name =
        binding.kind === "user"
          ? DESTINATION_COPY.user
          : (registeredWorkspaces.find(workspace => workspace.id === binding.workspaceId)?.name ??
            session.workspaceLabel ??
            binding.workspaceId);
      notifyUser(workspaceSwitchFeedback(name, session.title));
    }
    jumpToSession({
      sessionId: session.sessionId,
      agentName: session.agentName,
      workspaceId: session.workspaceId,
    });
  };

  return {
    destinationWindowId,
    goToTab: (windowId: string) => {
      close();
      void coordinator.userActivateWindow(windowId);
    },
    landSession,
    openSession: (session: OsPaletteSessionResult) => {
      if (session.busy) return;
      if (destinationWindowId !== null) {
        consumeChooseSessionTerminalQuote(session.sessionId);
        close();
        pickDestination({
          app: "session",
          instanceKey: session.sessionId,
          // The All projects desktop binds the window to the session's own scope.
          route:
            dataScope?.kind === "all"
              ? boundRoute({ binding: ownerDataScope(session.workspaceId), route: session.route })
              : session.route,
        });
        return;
      }
      landSession(session);
      close();
    },
    pickDestination,
    selectWorktree: (entry: OsPaletteWorktreeResult) => {
      close();
      entities.selectWorktree(entry);
    },
  };
}

export type OsPaletteLandingActions = ReturnType<typeof useOsPaletteLandingActions>;
