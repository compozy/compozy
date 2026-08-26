/** The journal is one per project and never closes, so it pins to the strip. */
export const TERMINAL_JOURNAL_TAB: unique symbol = Symbol("terminal-journal-tab");

export type TerminalTabId = string | typeof TERMINAL_JOURNAL_TAB;

export function terminalTabDomId(idBase: string, tab: TerminalTabId): string {
  return `${idBase}-tab-${tab === TERMINAL_JOURNAL_TAB ? "journal" : tab}`;
}

export function terminalPanelDomId(idBase: string): string {
  return `${idBase}-panel`;
}
