import { createStoreLogic } from "@xstate/store";

import { buildTerminalQuote, type TerminalQuote } from "@/systems/terminal/parts";

import { sessionStore } from "../stores/session-store";

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
 * Returns the canonical block so the caller can also put it in the composer
 * text — the block is what actually reaches the agent; the on-screen chip is
 * how a person sees and removes it.
 */
export function stageSessionTerminalQuote(input: StageTerminalQuoteInput): TerminalQuote {
  const quote = buildTerminalQuote({
    terminalId: input.terminalId,
    fromLine: input.fromLine,
    lines: input.lines,
  });
  // The chip and the block are staged together, here, at the gesture — not
  // reconciled afterwards by an effect. The block is what actually reaches the
  // agent; a chip with nothing behind it would send a message about an excerpt
  // that is not in the message. Staging a second quote replaces the first
  // rather than stacking them, and staging the same one twice changes nothing.
  const previous = sessionTerminalQuoteStore.getSnapshot().context.quotes[input.sessionId] ?? null;
  sessionTerminalQuoteStore.trigger.staged({ sessionId: input.sessionId, quote });
  sessionStore.trigger.composerDraftChanged({
    sessionId: input.sessionId,
    text: applyTerminalQuoteToDraft(readDraft(input.sessionId), previous?.text ?? null, quote.text),
  });
  return quote;
}

/** Takes the quote back out, leaving whatever was typed around it. */
export function clearSessionTerminalQuote(sessionId: string): void {
  const quote = sessionTerminalQuoteStore.getSnapshot().context.quotes[sessionId];
  sessionTerminalQuoteStore.trigger.removed({ sessionId });
  if (!quote) return;
  sessionStore.trigger.composerDraftChanged({
    sessionId,
    text: stripTerminalQuote(readDraft(sessionId), quote),
  });
}

function readDraft(sessionId: string): string {
  return sessionStore.getSnapshot().context.drafts[sessionId] ?? "";
}

/** Removes the block from a composer draft, leaving whatever was annotated. */
export function stripTerminalQuote(text: string, quote: TerminalQuote): string {
  return stripQuoteBlock(text, quote.text);
}

function stripQuoteBlock(text: string, block: string): string {
  if (block === "" || !text.includes(block)) return text;
  return text.replace(block, "").replace(/^\n+/, "").replace(/\n+$/, "");
}

/**
 * Puts the canonical block in the draft, exactly once.
 *
 * The block is what actually reaches the agent, so staging a quote has to write
 * it into the composer — a chip with nothing behind it would send a message
 * about a terminal excerpt that is not in the message. Replacing one quote with
 * another removes the first rather than stacking them, and staging the same
 * quote twice changes nothing.
 */
export function applyTerminalQuoteToDraft(
  draft: string,
  previousBlock: string | null,
  nextBlock: string
): string {
  const base = previousBlock ? stripQuoteBlock(draft, previousBlock) : draft;
  if (base.includes(nextBlock)) return base;
  return base === "" ? nextBlock : `${base}\n\n${nextBlock}`;
}
