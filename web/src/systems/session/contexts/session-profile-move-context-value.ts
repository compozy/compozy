import { createContext } from "react";

import type { SessionPayload } from "../types";

/** One profile a session can move to: an active profile other than its own. */
export interface SessionProfileMoveTarget {
  id: string;
  name: string;
}

/**
 * The profiles a row can move its session to, and the request that opens the
 * Move to profile dialog for one of them. Absent (null) outside a host; the
 * row menu then hides the verb.
 */
export interface SessionProfileMoveHandle {
  targets: readonly SessionProfileMoveTarget[];
  request: (source: SessionPayload, target: SessionProfileMoveTarget) => void;
}

export const SessionProfileMoveContext = createContext<SessionProfileMoveHandle | null>(null);
