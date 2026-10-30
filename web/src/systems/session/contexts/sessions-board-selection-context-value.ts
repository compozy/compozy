import { createContext } from "react";

import type { SessionSelectionVerbs } from "../hooks/use-session-list-bulk-selection";
import type { SessionSelectionStore } from "../hooks/use-session-selection";
import type { SessionListGroupRowsStore } from "./session-list-group-selection-context-value";

/**
 * The board's one selection store. Each card, column and lane header subscribes to
 * its own membership, so a toggle re-renders only what it changes, never every card.
 */
export const SessionsBoardSelectionStoreContext = createContext<SessionSelectionStore | null>(null);

/** Where each board column (and the open Archive bar) publishes the cards it shows, in order. */
export const SessionsBoardRowsStoreContext = createContext<SessionListGroupRowsStore | null>(null);

/**
 * The selection's verbs and counts, read only by a selected card's open context menu:
 * they change with every toggle, so no card reads them while rendering.
 */
export const SessionsBoardSelectionVerbsContext = createContext<SessionSelectionVerbs | null>(null);
