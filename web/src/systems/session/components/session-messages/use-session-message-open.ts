import { use } from "react";

import { SubagentNavigationContext } from "@/systems/session/contexts/session-subagents-context-value";

import type { SessionMessageOpen } from "./session-message-party";

/**
 * Opens the session a message names through the window's navigation (the
 * subagent drill-in's `openSession`): this window, or a new one with ⌘/Ctrl.
 * `undefined` where no window can open it, so the title renders without a link.
 */
export function useSessionMessageOpen(): SessionMessageOpen | undefined {
  const navigation = use(SubagentNavigationContext);
  if (!navigation) return undefined;
  return (party, options) =>
    navigation.openSession(
      {
        sessionId: party.sessionId,
        agentName: party.agentName ?? "",
        workspaceId: party.workspaceId,
      },
      options
    );
}
