import { createContext } from "react";

import type { terminalStoreLogic } from "../stores/terminal-store";

export type TerminalStore = ReturnType<typeof terminalStoreLogic.createStore>;

/**
 * The store handle a Terminal app provides.
 *
 * Kept apart from the provider component so the context identity survives a
 * fast-refresh of the component that supplies it.
 */
export const TerminalStoreContext = createContext<TerminalStore | null>(null);
