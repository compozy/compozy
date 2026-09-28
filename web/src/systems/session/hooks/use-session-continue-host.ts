import { useState } from "react";

import type { SessionContinueRequest } from "../contexts/session-continue-context-value";
import { isUserControllableSession } from "../lib/session-running";
import type { SessionPayload } from "../types";

interface ContinueTarget {
  source: SessionPayload;
  /** Each open starts fresh: a new measurement and a new idempotency key. */
  nonce: number;
}

export interface UseSessionContinueHostInput {
  /** Workspace used when a source payload does not name its own. */
  workspaceId: string;
  /** The session a source-less request (the transcript marker, the topbar) continues. */
  currentSession?: SessionPayload;
}

/**
 * The Continue dialog's open state for one surface. Split from the host
 * component so a surface that publishes chrome outside its own tree (the
 * window topbar) holds the request function before rendering the host.
 */
export function useSessionContinueHost({
  workspaceId,
  currentSession,
}: UseSessionContinueHostInput) {
  const [target, setTarget] = useState<ContinueTarget | null>(null);
  const [open, setOpen] = useState(false);

  const request: SessionContinueRequest = source => {
    const next = source ?? currentSession;
    if (!next || !isUserControllableSession(next)) return;
    setTarget(previous => ({ source: next, nonce: (previous?.nonce ?? 0) + 1 }));
    setOpen(true);
  };

  return { workspaceId, target, open, setOpen, request };
}

export type SessionContinueHostState = ReturnType<typeof useSessionContinueHost>;
