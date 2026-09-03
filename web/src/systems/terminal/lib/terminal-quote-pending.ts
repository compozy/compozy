import { createStoreLogic } from "@xstate/store";

import type { TerminalQuote } from "./terminal-quote";

interface TerminalQuotePendingState {
  pending: TerminalQuote | null;
}

type TerminalQuotePendingEvents = {
  held: { quote: TerminalQuote };
  cleared: Record<string, never>;
};

const terminalQuotePendingLogic = createStoreLogic<
  TerminalQuotePendingState,
  TerminalQuotePendingEvents
>({
  context: { pending: null },
  on: {
    held: (_context, event) => ({ pending: event.quote }),
    cleared: context => {
      if (!context.pending) return undefined;
      return { pending: null };
    },
  },
});

const terminalQuotePendingStore = terminalQuotePendingLogic.createStore();

/**
 * Holds a quote until create produces a session id.
 *
 * Start-a-session lands here. Choose-a-session stages onto the chosen id
 * through `stageChosenSessionTerminalQuote` and must not use this slot.
 */
export function holdPendingTerminalQuote(quote: TerminalQuote): void {
  terminalQuotePendingStore.trigger.held({ quote });
}

export function peekPendingTerminalQuote(): TerminalQuote | null {
  return terminalQuotePendingStore.getSnapshot().context.pending;
}

export function takePendingTerminalQuote(): TerminalQuote | null {
  const quote = terminalQuotePendingStore.getSnapshot().context.pending;
  terminalQuotePendingStore.trigger.cleared({});
  return quote;
}

/** Drops a held quote without staging it — create cancel/dismiss. */
export function clearPendingTerminalQuote(): void {
  terminalQuotePendingStore.trigger.cleared({});
}
