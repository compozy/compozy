import type { ReactNode } from "react";

import { useSessionTranscriptThreadState } from "../hooks/use-session-transcript-thread-messages";
import { deriveTurnOutcomes, type SessionTurnOutcomes } from "./session-turn-outcomes";
import { SessionTurnOutcomesContext } from "./session-turn-outcomes-context-value";

// Cached by the message list's identity: a pure function of the loaded thread.
const outcomesByMessages = new WeakMap<object, SessionTurnOutcomes>();

function outcomesFor(messages: readonly object[]): SessionTurnOutcomes {
  const cached = outcomesByMessages.get(messages);
  if (cached) return cached;
  const outcomes = deriveTurnOutcomes(messages);
  outcomesByMessages.set(messages, outcomes);
  return outcomes;
}

/** Makes every turn's recorded end available to the messages that render its calls. */
export function SessionTurnOutcomesProvider({ children }: { children: ReactNode }) {
  const { messages } = useSessionTranscriptThreadState();
  return (
    <SessionTurnOutcomesContext.Provider value={outcomesFor(messages)}>
      {children}
    </SessionTurnOutcomesContext.Provider>
  );
}
