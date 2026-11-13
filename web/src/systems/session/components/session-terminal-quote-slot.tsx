"use client";

import { TerminalQuoteBlock, type TerminalQuote } from "@/systems/terminal/parts";

import { useSessionTerminalQuote } from "../hooks/use-session-terminal-quote";
import { clearSessionTerminalQuote } from "../lib/session-terminal-quote";

export interface SessionTerminalQuoteSlotProps {
  sessionId: string;
}

/**
 * Every staging builds a new quote object, so object identity tells two
 * excerpts apart even when they share a terminal and line range.
 */
const stagedQuoteKeys = new WeakMap<TerminalQuote, number>();
let nextStagedQuoteKey = 0;

function stagedQuoteKey(quote: TerminalQuote): number {
  let key = stagedQuoteKeys.get(quote);
  if (key === undefined) {
    nextStagedQuoteKey += 1;
    key = nextStagedQuoteKey;
    stagedQuoteKeys.set(quote, key);
  }
  return key;
}

/**
 * The terminal excerpt waiting in the composer stack.
 *
 * The chip is how a person sees where the excerpt came from and takes it back
 * out. The envelope stays in the quote store until send.
 */
export function SessionTerminalQuoteSlot({ sessionId }: SessionTerminalQuoteSlotProps) {
  const quote = useSessionTerminalQuote(sessionId);
  if (!quote) return null;
  // A newly staged excerpt mounts fresh, so a collapse lasts only for the quote it was made on.
  return (
    <TerminalQuoteBlock
      key={stagedQuoteKey(quote)}
      onRemove={() => clearSessionTerminalQuote(sessionId)}
      quote={quote}
    />
  );
}
