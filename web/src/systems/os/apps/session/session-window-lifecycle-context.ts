import { createContext } from "react";

import type { useSessionListLifecycleActions } from "../../hooks/use-session-list-lifecycle-actions";

/** The lifecycle confirmations a session window owns, single or batched. */
export type SessionWindowLifecycle = ReturnType<typeof useSessionListLifecycleActions>;

/** Window-owned lifecycle state; `null` outside a `SessionWindowLifecycleHost`. */
export const SessionWindowLifecycleContext = createContext<SessionWindowLifecycle | null>(null);
