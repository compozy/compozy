import { notifyUser } from "@/lib/user-feedback";

import { sessionStore } from "../stores/session-store";

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
export function sendFirstPrompt(sessionId: string, send: (text: string) => void): void {
  const prompt = sessionStore.getSnapshot().context.firstPrompts[sessionId];
  if (!prompt || prompt.claimed) {
    return;
  }
  sessionStore.trigger.firstPromptClaimed({ sessionId });
  try {
    send(prompt.text);
    sessionStore.trigger.firstPromptSent({ sessionId });
  } catch {
    sessionStore.trigger.firstPromptRecovered({ sessionId });
    notifyUser({ message: FIRST_PROMPT_SEND_FAILED, tone: "error" });
  }
}
