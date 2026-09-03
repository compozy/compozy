import { createStoreLogic } from "@xstate/store";

import type { TerminalQuote } from "./terminal-quote";

interface TerminalQuoteChooseState {
  pending: TerminalQuote | null;
}

type TerminalQuoteChooseEvents = {
  held: { quote: TerminalQuote };
  cleared: Record<string, never>;
};

const terminalQuoteChooseLogic = createStoreLogic<
  TerminalQuoteChooseState,
  TerminalQuoteChooseEvents
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

const terminalQuoteChooseStore = terminalQuoteChooseLogic.createStore();

/**
 * Holds a quote for the session picker.
 *
 * Choose-a-session must not occupy the create-only pending slot. After the
 * operator picks a session, `takeChooseSessionTerminalQuote` returns this
 * quote so the host can `stageChosenSessionTerminalQuote`.
 */
export function holdChooseSessionTerminalQuote(quote: TerminalQuote): void {
  terminalQuoteChooseStore.trigger.held({ quote });
}

export function peekChooseSessionTerminalQuote(): TerminalQuote | null {
  return terminalQuoteChooseStore.getSnapshot().context.pending;
}

export function takeChooseSessionTerminalQuote(): TerminalQuote | null {
  const quote = terminalQuoteChooseStore.getSnapshot().context.pending;
  terminalQuoteChooseStore.trigger.cleared({});
  return quote;
}

/** Drops a held choose-quote without staging it — picker cancel/dismiss. */
export function clearChooseSessionTerminalQuote(): void {
  terminalQuoteChooseStore.trigger.cleared({});
}
