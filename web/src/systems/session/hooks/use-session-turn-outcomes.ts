import { use } from "react";

import { emptyTurnOutcomes, type SessionTurnOutcomes } from "../lib/session-turn-outcomes";
import { SessionTurnOutcomesContext } from "../lib/session-turn-outcomes-context-value";

/** The loaded thread's turn end records; empty outside a thread. */
export function useSessionTurnOutcomes(): SessionTurnOutcomes {
  return use(SessionTurnOutcomesContext) ?? emptyTurnOutcomes();
}
