import { createContext } from "react";

import type { SessionOriginView } from "../lib/session-origin";

/**
 * The origin of the session a transcript renders, for the divider before its
 * first own message. Provided by the session window; absent for roots and for
 * surfaces that render a transcript without its session.
 */
export interface SessionOriginContextValue {
  origin: SessionOriginView;
  onOpenSource?: (sessionId: string) => void;
}

export const SessionOriginContext = createContext<SessionOriginContextValue | null>(null);
