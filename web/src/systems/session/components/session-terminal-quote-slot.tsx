"use client";

import { TerminalQuoteBlock } from "@/systems/terminal/parts";

import { useSessionTerminalQuote } from "../hooks/use-session-terminal-quote";
import { clearSessionTerminalQuote } from "../lib/session-terminal-quote";

export interface SessionTerminalQuoteSlotProps {
  sessionId: string;
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
  // A new excerpt mounts fresh, so a collapse lasts only for the quote it was made on.
  return (
    <TerminalQuoteBlock
      key={`${quote.terminalId}:${quote.fromLine}-${quote.toLine}`}
      onRemove={() => clearSessionTerminalQuote(sessionId)}
      quote={quote}
    />
  );
}
