import { use } from "react";

import type { SubagentRoster } from "../lib/subagent-roster";
import { useSubagentRoster } from "./use-subagent-roster";

import {
  SessionSubagentsContext,
  type SessionSubagentsContextValue,
} from "../contexts/session-subagents-context-value";

/** The rendered session's subagent roster and drill-in (empty outside a session thread). */
export function useSubagentRosterContext(): SessionSubagentsContextValue {
  return use(SessionSubagentsContext);
}

/** The rendered session's whole roster, for the one-per-thread banner and status line. */
export function useSessionSubagentRoster(): SubagentRoster {
  const { workspaceId, sessionId } = useSubagentRosterContext();
  return useSubagentRoster(workspaceId, sessionId);
}
