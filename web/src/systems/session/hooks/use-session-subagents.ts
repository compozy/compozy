import { use } from "react";

import {
  SessionSubagentsContext,
  type SessionSubagentsContextValue,
} from "../contexts/session-subagents-context-value";

/** The rendered session's subagent roster and drill-in (empty outside a session thread). */
export function useSessionSubagents(): SessionSubagentsContextValue {
  return use(SessionSubagentsContext);
}
