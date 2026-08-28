import type { UIMessage } from "ai";

import { parseTerminalQuote, type TerminalQuote } from "@/systems/terminal/parts";

import { peekSessionTerminalQuote } from "./session-terminal-quote";

const ENVELOPE_CLOSE = "\n</terminal_context>";

/**
 * The bytes that reach the agent: envelope first, then whatever the person
 * typed around the chip.
 */
export function composeQuotedPrompt(annotation: string, quote: TerminalQuote | null): string {
  if (!quote) return annotation;
  if (annotation.includes(quote.text)) return annotation;
  const trimmed = annotation.trim();
  return trimmed === "" ? quote.text : `${quote.text}\n\n${trimmed}`;
}

/**
 * Inverse of `composeQuotedPrompt`: envelope identity vs annotation text.
 *
 * Recovery and queued-prompt edit use this so the editor never receives
 * `<terminal_context>`.
 */
export function splitQuotedPrompt(text: string): {
  annotation: string;
  quote: TerminalQuote | null;
} {
  const exact = parseTerminalQuote(text);
  if (exact) return { annotation: "", quote: exact };
  if (!text.startsWith("<terminal_context ")) {
    return { annotation: text, quote: null };
  }
  const closeAt = text.indexOf(ENVELOPE_CLOSE);
  if (closeAt === -1) return { annotation: text, quote: null };
  const envelope = text.slice(0, closeAt + ENVELOPE_CLOSE.length);
  const quote = parseTerminalQuote(envelope);
  if (!quote) return { annotation: text, quote: null };
  let annotation = text.slice(envelope.length);
  if (annotation.startsWith("\n\n")) annotation = annotation.slice(2);
  else if (annotation.startsWith("\n")) annotation = annotation.slice(1);
  return { annotation, quote };
}

export function composeSessionPromptWithTerminalQuote(
  sessionId: string,
  annotation: string
): string {
  return composeQuotedPrompt(annotation, peekSessionTerminalQuote(sessionId));
}

/**
 * Puts the sourced envelope on the prompt the transport is about to send.
 *
 * The composer field stays as the annotation. The agent still receives the
 * same block `compozy terminal quote` would have printed.
 */
export function applyTerminalQuoteToPromptMessage(
  sessionId: string,
  message: UIMessage
): UIMessage {
  const quote = peekSessionTerminalQuote(sessionId);
  if (!quote) return message;
  let applied = false;
  const parts = message.parts.map(part => {
    if (part.type !== "text" || applied) return part;
    applied = true;
    return { ...part, text: composeQuotedPrompt(part.text, quote) };
  });
  if (!applied) {
    return { ...message, parts: [{ type: "text", text: quote.text }, ...message.parts] };
  }
  return { ...message, parts };
}
