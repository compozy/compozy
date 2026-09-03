import { notifyUser } from "@/lib/user-feedback";
import { consumeChooseSessionTerminalQuote } from "@/systems/session";
import type { WorkspacePayload } from "@/systems/workspace";

import { workspaceSwitchFeedback } from "../lib/cmd-palette-feedback";
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
  runtimeWorkspaceId: string | null;
}

export function useOsPaletteLandingActions({
  close,
  destinationWindowId,
  entities,
  registeredWorkspaces,
  runtimeWorkspaceId,
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
    consumeChooseSessionTerminalQuote(session.sessionId);
    if (session.workspaceId !== "" && session.workspaceId !== runtimeWorkspaceId) {
      const name =
        registeredWorkspaces.find(workspace => workspace.id === session.workspaceId)?.name ??
        session.workspaceLabel ??
        session.workspaceId;
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
      if (destinationWindowId !== null) {
        consumeChooseSessionTerminalQuote(session.sessionId);
        close();
        pickDestination({ app: "session", instanceKey: session.sessionId, route: session.route });
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
