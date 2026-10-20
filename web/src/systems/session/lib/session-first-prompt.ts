import { notifyUser } from "@/lib/user-feedback";

import { parseTerminalQuote } from "@/systems/terminal/parts";

import { stageSessionTerminalQuote } from "./session-terminal-quote";
import { sessionStore, type SessionFirstPrompt } from "../stores/session-store";

/** What the operator is told when the runtime refuses the message they typed. */
export const FIRST_PROMPT_SEND_FAILED =
  "Couldn't send the first message. It is waiting in the composer.";

/**
 * Hands a session's queued first message to the runtime, at most once.
 *
 * The guard is the pair of statements at the top: a fresh read followed by the
 * claim, with nothing awaited between them. JavaScript runs that pair to
 * completion, so a second attempt — a StrictMode double-invoke, a remount, two
 * mounted threads — reads an already-claimed slot and returns without sending.
 *
 * A refusal never costs the operator their words: the message becomes an
 * ordinary composer draft, visible and editable, instead of disappearing.
 */
export function sendFirstPrompt(
  sessionId: string,
  send: (text: string, prompt: SessionFirstPrompt) => void | Promise<void>,
  signal?: AbortSignal
): void {
  const prompt = sessionStore.getSnapshot().context.firstPrompts[sessionId];
  if (!prompt || prompt.claimed || signal?.aborted) {
    return;
  }
  sessionStore.trigger.firstPromptClaimed({ sessionId });
  const quote = parseTerminalQuote(prompt.text);
  if (quote) {
    stageSessionTerminalQuote({
      sessionId,
      terminalId: quote.terminalId,
      fromLine: quote.fromLine,
      lines: quote.lines,
    });
    sessionStore.trigger.firstPromptSent({ sessionId });
    return;
  }
  try {
    const result = send(prompt.text, prompt);
    if (result) {
      const release = () => sessionStore.trigger.firstPromptReleased({ sessionId });
      signal?.addEventListener("abort", release, { once: true });
      void result.then(
        () => {
          signal?.removeEventListener("abort", release);
          if (!signal?.aborted) sessionStore.trigger.firstPromptSent({ sessionId });
        },
        () => {
          signal?.removeEventListener("abort", release);
          if (!signal?.aborted) recoverFirstPrompt(sessionId);
        }
      );
    } else {
      sessionStore.trigger.firstPromptSent({ sessionId });
    }
  } catch {
    recoverFirstPrompt(sessionId);
  }
}

function recoverFirstPrompt(sessionId: string) {
  sessionStore.trigger.firstPromptRecovered({ sessionId });
  notifyUser({ message: FIRST_PROMPT_SEND_FAILED, tone: "error" });
}
