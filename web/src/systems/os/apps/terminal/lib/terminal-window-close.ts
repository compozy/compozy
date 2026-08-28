import { shouldKeepTerminalJournalHost } from "@/systems/terminal";

export type TerminalCloseHostDecision =
  | { kind: "noop" }
  | { kind: "retarget"; terminalId: string }
  | { kind: "keep" }
  | { kind: "close" };

/**
 * What the host does after a terminal close succeeds.
 *
 * The journal tab does not change the route, so "last PTY" is not "close the
 * app" while that tab is showing. Remaining terminals still retarget as before.
 */
export function decideTerminalCloseHost(input: {
  closedId: string;
  routedId: string | null;
  remaining: readonly { id: string }[];
  journalVisible: boolean;
}): TerminalCloseHostDecision {
  const keep = shouldKeepTerminalJournalHost({
    remainingTerminalCount: input.remaining.length,
    journalVisible: input.journalVisible,
  });
  if (input.routedId !== input.closedId) return { kind: "noop" };
  const next = input.remaining[0];
  if (next) return { kind: "retarget", terminalId: next.id };
  return keep ? { kind: "keep" } : { kind: "close" };
}
