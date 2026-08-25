"use client";

import { TerminalQuoteBlock } from "@/systems/terminal/parts";

import { useSessionTerminalQuote } from "../hooks/use-session-terminal-quote";
import { clearSessionTerminalQuote } from "../lib/session-terminal-quote";

export interface SessionTerminalQuoteSlotProps {
  sessionId: string;
}

/**
 * The terminal excerpt waiting above the composer.
 *
 * The chip is how a person sees where the excerpt came from and takes it back
 * out; the canonical block already sits in the draft, put there when the quote
 * was staged. Removing it here strips exactly that block and leaves whatever
 * was typed around it.
 */
export function SessionTerminalQuoteSlot({ sessionId }: SessionTerminalQuoteSlotProps) {
  const quote = useSessionTerminalQuote(sessionId);
  if (!quote) return null;
  return <TerminalQuoteBlock onRemove={() => clearSessionTerminalQuote(sessionId)} quote={quote} />;
}
