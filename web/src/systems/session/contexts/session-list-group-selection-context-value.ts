import { createContext } from "react";

import type { SessionSelectionDrag } from "../hooks/use-session-list-dnd";
import type { SessionRowSelection } from "../hooks/use-session-selection";
import type { sessionListGroupRowsLogic } from "../lib/session-list-group-rows";

export type SessionListGroupRowsStore = ReturnType<typeof sessionListGroupRowsLogic.createStore>;

/**
 * The grouped list's one selection, handed to every group: each group publishes
 * its rows to `rows`, renders `selection` on them, and wires `drag` only where
 * the dragged selection moves within that group's pair.
 */
export interface SessionListGroupSelection {
  rows: SessionListGroupRowsStore;
  selection: SessionRowSelection;
  drag?: SessionSelectionDrag;
}

export const SessionListGroupSelectionContext = createContext<SessionListGroupSelection | null>(
  null
);
