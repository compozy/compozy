import { createContext } from "react";

import type { SessionPayload } from "../types";

/**
 * Provided by `SessionDeriveHost`: opens the Continue dialog for a source
 * session (Fork's request travels beside it in `SessionForkContext`). Surfaces that belong to one
 * session (the transcript's provider-error marker) call it without a source:
 * the host that mounted them knows which session they belong to. Absent (null)
 * outside a host, where no dialog could open — the entry points hide.
 */
export type SessionContinueRequest = (source?: SessionPayload) => void;

export const SessionDeriveContext = createContext<SessionContinueRequest | null>(null);
