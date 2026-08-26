/** The journal is one per project and never closes, so it pins to the strip. */
export const TERMINAL_JOURNAL_TAB: unique symbol = Symbol("terminal-journal-tab");

export type TerminalTabId = string | typeof TERMINAL_JOURNAL_TAB;
