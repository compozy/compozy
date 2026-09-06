import { createContext } from "react";

import type { SessionTurnOutcomes } from "./session-turn-outcomes";

/** Per-turn end records across the loaded thread; `null` outside a thread. */
export const SessionTurnOutcomesContext = createContext<SessionTurnOutcomes | null>(null);
