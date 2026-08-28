import { createStoreLogic } from "@xstate/store";

import {
  buildTerminalQuote,
  clearChooseSessionTerminalQuote,
  clearPendingTerminalQuote,
  holdPendingTerminalQuote,
  peekPendingTerminalQuote,
  takeChooseSessionTerminalQuote,
  takePendingTerminalQuote,
  type TerminalQuote,
} from "@/systems/terminal/parts";

/**
 * The terminal excerpt waiting in a session's composer.
 *
 * Staged per session, because a quote belongs to the conversation it was sent
 * to. The store holds one at a time: the gesture is "send this to the
 * conversation", not "collect excerpts".
 */
interface SessionTerminalQuoteState {
  quotes: Record<string, TerminalQuote>;
}

type SessionTerminalQuoteEvents = {
  staged: { sessionId: string; quote: TerminalQuote };
  removed: { sessionId: string };
};

const sessionTerminalQuoteLogic = createStoreLogic<
  SessionTerminalQuoteState,
  SessionTerminalQuoteEvents
>({
  context: { quotes: {} },
  on: {
    staged: (context, event) => ({
      quotes: { ...context.quotes, [event.sessionId]: event.quote },
    }),
    removed: (context, event) => {
      if (!context.quotes[event.sessionId]) return undefined;
      const quotes = { ...context.quotes };
      delete quotes[event.sessionId];
      return { quotes };
    },
  },
});

/**
 * One store for the whole app.
 *
 * The gesture starts in a terminal window and lands in a session composer —
 * two different React trees — so the handoff cannot live in either one's
 * provider.
 */
export const sessionTerminalQuoteStore = sessionTerminalQuoteLogic.createStore();

export interface StageTerminalQuoteInput {
  sessionId: string;
  terminalId: string;
  fromLine: number;
  lines: readonly string[];
}

/**
 * Stages a selection for a session.
 *
 * The chip is how a person sees the excerpt. The envelope stays in this store
 * until send, where it is concatenated with the annotation. Writing the XML
 * into the composer draft would show the agent envelope as if the person typed
 * it.
 */
export function stageSessionTerminalQuote(input: StageTerminalQuoteInput): TerminalQuote {
  const quote = buildTerminalQuote({
    terminalId: input.terminalId,
    fromLine: input.fromLine,
    lines: input.lines,
  });
  sessionTerminalQuoteStore.trigger.staged({ sessionId: input.sessionId, quote });
  return quote;
}

/** Takes the quote back out of the session. */
export function clearSessionTerminalQuote(sessionId: string): void {
  sessionTerminalQuoteStore.trigger.removed({ sessionId });
}

/** Removes the visual handoff without changing the submitted composer text. */
export function discardSessionTerminalQuote(sessionId: string): void {
  sessionTerminalQuoteStore.trigger.removed({ sessionId });
}

export function peekSessionTerminalQuote(sessionId: string): TerminalQuote | null {
  return sessionTerminalQuoteStore.getSnapshot().context.quotes[sessionId] ?? null;
}

export {
  clearChooseSessionTerminalQuote,
  clearPendingTerminalQuote,
  holdPendingTerminalQuote,
  peekPendingTerminalQuote,
  takePendingTerminalQuote,
};

/**
 * Stages a quote onto a session the operator already chose.
 *
 * Host integration: after the session picker returns an id, call this with
 * that id and the quote handed to `openSessionPicker`. Never wait for a
 * thread to mount.
 */
export function stageChosenSessionTerminalQuote(
  sessionId: string,
  quote: TerminalQuote
): TerminalQuote {
  return stageSessionTerminalQuote({
    sessionId,
    terminalId: quote.terminalId,
    fromLine: quote.fromLine,
    lines: quote.lines,
  });
}

/**
 * Claims the choose-held quote onto the session the operator just picked.
 *
 * Exact-once: take clears the slot. Dismiss must call
 * `clearChooseSessionTerminalQuote` instead of this.
 */
export function consumeChooseSessionTerminalQuote(sessionId: string): TerminalQuote | null {
  const quote = takeChooseSessionTerminalQuote();
  if (!quote) return null;
  return stageChosenSessionTerminalQuote(sessionId, quote);
}

/**
 * Claims the create-held quote into an in-flight attempt.
 *
 * Call this when submit becomes inevitable — the same moment `firstMessage`
 * is captured. Success must stage this value, never a later global slot.
 */
export function claimPendingTerminalQuoteForCreate(): TerminalQuote | null {
  return takePendingTerminalQuote();
}

/**
 * Stages a create-held quote onto the session that just received an id.
 *
 * Immediate consume only. Create success after await must pass the quote
 * captured at submit, not call this.
 */
export function stagePendingTerminalQuoteForSession(sessionId: string): TerminalQuote | null {
  const quote = takePendingTerminalQuote();
  if (!quote) return null;
  return stageChosenSessionTerminalQuote(sessionId, quote);
}

/**
 * Puts a failed attempt's quote back only when nothing newer is waiting.
 *
 * Empty slot → restore. Same envelope still held → restore (idempotent).
 * A different pending quote is a newer intent and must not be overwritten.
 */
export function restorePendingTerminalQuoteAfterFailedCreate(quote: TerminalQuote | null): void {
  if (!quote) return;
  const current = peekPendingTerminalQuote();
  if (current !== null && current.text !== quote.text) return;
  holdPendingTerminalQuote(quote);
}
