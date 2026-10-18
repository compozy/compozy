import { createContext } from "react";

import type { SessionInteractionRecord } from "../types";

export interface SessionRuntimeRenderContextValue {
  durableMessageIds: ReadonlySet<string>;
  /**
   * Decisions the daemon settled without a transcript answer (expired at a daemon
   * restart), keyed by the provider request id the transcript's permission/clarify
   * parts carry. Empty when nothing on screen is undecided.
   */
  expiredInteractions: ReadonlyMap<string, SessionInteractionRecord>;
  /**
   * Decisions the daemon applied, keyed the same way, carrying `resolved_by` for the
   * transcript's receipts. Empty when nothing on screen is decided or nothing was read.
   */
  resolvedInteractions: ReadonlyMap<string, SessionInteractionRecord>;
  resetRuntime?: () => void;
  rewindBlocked?: boolean;
  /**
   * The daemon reports a turn in flight for this session (its detail read model),
   * including one started outside this page — the CLI, another window, a reload.
   */
  sessionRunning?: boolean;
  sessionId: string;
  workspaceId: string;
}

export const SessionRuntimeRenderContext = createContext<SessionRuntimeRenderContextValue | null>(
  null
);
