export interface TerminalJournalHostRetention {
  remainingTerminalCount: number;
  journalVisible: boolean;
}

/**
 * Whether the terminal host should stay mounted after a close.
 *
 * The journal is project history, not a property of the last PTY. Closing the
 * last live terminal must not take the journal with it while that tab is open.
 */
export function shouldKeepTerminalJournalHost(input: TerminalJournalHostRetention): boolean {
  return input.journalVisible || input.remainingTerminalCount > 0;
}
