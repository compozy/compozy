import { createContext } from "react";

import type { SessionPayload } from "../types";

/** The durable user message a fork cuts through, and its text for the fork-point sentence. */
export interface SessionForkPoint {
  messageId: string;
  messageText: string;
}

/**
 * Opens the Fork dialog for a source session — the whole session, or through
 * `point` when "Fork from here" was clicked on a message. Surfaces that belong
 * to one session (the transcript) call it without a source: the host knows
 * which session they belong to. Absent (null) outside a host; entry points hide.
 */
export type SessionForkRequest = (source?: SessionPayload, point?: SessionForkPoint) => void;

export const SessionForkContext = createContext<SessionForkRequest | null>(null);
