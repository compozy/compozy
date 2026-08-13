import { useEffect } from "react";
import { useAui, useAuiState } from "@assistant-ui/react";
import { useSelector } from "@xstate/store-react";

import { sendFirstPrompt } from "../lib/session-first-prompt";
import { sessionStore } from "../stores/session-store";

interface UseSessionFirstPromptParams {
  sessionId: string;
  /** The same gate the Send control uses — a session that cannot prompt cannot receive one. */
  canPrompt: boolean;
}

/**
 * Delivers the message typed before this session existed.
 *
 * Session creation is a different surface from the composer, so the message
 * crosses between them through the session store rather than a navigation
 * parameter. Sending is a synchronization with the assistant-ui runtime — an
 * external system — which is why it belongs in an effect; exactly-once is
 * enforced by {@link sendFirstPrompt}, not by this hook.
 */
export function useSessionFirstPrompt({ sessionId, canPrompt }: UseSessionFirstPromptParams): void {
  const aui = useAui();
  // The runtime refuses messages while it is disabled or still loading history,
  // and refuses them silently. Waiting for it is what keeps the claim honest:
  // nothing is consumed until there is something that can accept it.
  const threadAccepts = useAuiState(state => !state.thread.isDisabled && !state.thread.isLoading);
  const prompt = useSelector(sessionStore, snapshot => snapshot.context.firstPrompts[sessionId]);

  useEffect(() => {
    if (!prompt || prompt.claimed || !canPrompt || !threadAccepts) {
      return;
    }
    // Appended straight to the thread rather than staged through the composer:
    // this is a message to send, not a draft, and it must not disturb whatever
    // the composer already holds.
    sendFirstPrompt(sessionId, text => aui.thread.append(text));
  }, [aui, canPrompt, prompt, sessionId, threadAccepts]);
}
