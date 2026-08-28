import {
  copySourcedTerminalQuote,
  terminalQuoteFromSelection,
  type TerminalQuote,
} from "@/systems/terminal/parts";

import { stageChosenSessionTerminalQuote } from "./session-terminal-quote";

export interface TerminalQuoteSelection {
  startLine: number;
  text: string;
}

export interface TerminalQuoteHost {
  getActiveSessionId: () => string | null;
  /**
   * Opens the session picker with the quote the chosen session must receive.
   * After pick, call `stageChosenSessionTerminalQuote(sessionId, quote)`.
   */
  openSessionPicker: (quote: TerminalQuote) => void;
  startSessionWithQuote: (quote: TerminalQuote) => void;
  activateSession: (sessionId: string) => void;
}

/**
 * The quote action API the terminal window / pipe pane must call.
 *
 * Host seam (do not absorb those files here — post-cherry-pick):
 * - `onCopySelection` → `actions.onCopySelection` / sourced clipboard bytes
 * - `onChooseSession(terminalId, selection)` → picker with the quote, then
 *   `stageChosenSessionTerminalQuote(chosenId, quote)` — never thread mount
 * - `onStartSession` → `openWithTerminalQuote(quote)` / `actions.onStartSession`
 * - `onOpenSettings` → `/settings/terminal`
 * - `terminal-pipe-log-pane.tsx` mounts `TerminalSelectionActions` and passes
 *   `firstLineNumber` as `selection.startLine`
 */
export function createTerminalQuoteHostActions(host: TerminalQuoteHost) {
  return {
    onSendSelection(terminalId: string, selection: TerminalQuoteSelection): void {
      const sessionId = host.getActiveSessionId();
      if (!sessionId) return;
      const quote = terminalQuoteFromSelection(terminalId, selection);
      stageChosenSessionTerminalQuote(sessionId, quote);
      host.activateSession(sessionId);
    },
    onCopySelection(terminalId: string, selection: TerminalQuoteSelection): void {
      void copySourcedTerminalQuote(terminalId, selection);
    },
    onChooseSession(terminalId: string, selection: TerminalQuoteSelection): void {
      host.openSessionPicker(terminalQuoteFromSelection(terminalId, selection));
    },
    onStartSession(terminalId: string, selection: TerminalQuoteSelection): void {
      host.startSessionWithQuote(terminalQuoteFromSelection(terminalId, selection));
    },
  };
}
