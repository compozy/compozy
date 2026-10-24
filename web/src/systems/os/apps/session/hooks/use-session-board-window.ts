import {
  useSessionCreateActions,
  useSessionsBoard,
  useSessionWindowViewState,
  type SessionBoardPayload,
} from "@/systems/session";
import { useDataScope } from "@/systems/workspace";

import { useSessionWindowOpenSession } from "../use-session-window-open-session";

/** Board reads plus the card gesture: back to List with that session open (US-011.AC-3). */
export function useSessionBoardWindow(windowId: string, sessionId: string | null) {
  const scope = useDataScope();
  const view = useSessionWindowViewState();
  const board = useSessionsBoard({ scope, hideChildren: view.hideChildren });
  const { openForAgent } = useSessionCreateActions();
  const openSession = useSessionWindowOpenSession({ windowId, sessionId: sessionId ?? undefined });
  const onOpen = (session: SessionBoardPayload) => {
    view.setView("list");
    openSession(
      {
        sessionId: session.id,
        agentName: session.agent_name,
        workspaceId: session.workspace_id ?? "",
      },
      { newWindow: false }
    );
  };
  return { view, board, openForAgent, onOpen };
}
