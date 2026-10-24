import { createContext } from "react";

/**
 * Provided by a session window for the one session it has open: clearing a
 * conversation resets that window's runtime thread, so only its own row in a
 * session list offers "Clear conversation". Absent (null) everywhere else.
 */
export interface SessionClearRequest {
  sessionId: string;
  /** The conversation has content and no turn or other control is in flight. */
  canClear: boolean;
  /** Opens the window's clear confirmation. */
  request: () => void;
}

export const SessionClearContext = createContext<SessionClearRequest | null>(null);
