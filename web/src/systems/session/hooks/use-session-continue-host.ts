import { useState } from "react";

import type { SessionContinueRequest } from "../contexts/session-continue-context-value";
import type { SessionForkPoint, SessionForkRequest } from "../contexts/session-fork-context-value";
import { isUserControllableSession } from "../lib/session-running";
import type { SessionPayload } from "../types";

interface DeriveTarget {
  /** Which dialog the host mounts: Continue (another agent) or Fork (same agent). */
  kind: "continue" | "fork";
  source: SessionPayload;
  /** Fork from here: the message the fork cuts through; absent for the whole session. */
  point?: SessionForkPoint;
  /** Each open starts fresh: a new measurement and a new idempotency key. */
  nonce: number;
}

export interface UseSessionContinueHostInput {
  /** Workspace used when a source payload does not name its own. */
  workspaceId: string;
  /** The session a source-less request (the transcript, the topbar) derives from. */
  currentSession?: SessionPayload;
}

/**
 * The derive dialogs' open state for one surface (Continue and Fork share it:
 * one dialog at a time). Split from the host component so a surface that
 * publishes chrome outside its own tree (the window topbar) holds the request
 * functions before rendering the host.
 */
export function useSessionContinueHost({
  workspaceId,
  currentSession,
}: UseSessionContinueHostInput) {
  const [target, setTarget] = useState<DeriveTarget | null>(null);
  const [open, setOpen] = useState(false);

  const openTarget = (
    kind: DeriveTarget["kind"],
    source: SessionPayload | undefined,
    point?: SessionForkPoint
  ) => {
    const next = source ?? currentSession;
    if (!next || !isUserControllableSession(next)) return;
    setTarget(previous => ({
      kind,
      source: next,
      ...(point ? { point } : {}),
      nonce: (previous?.nonce ?? 0) + 1,
    }));
    setOpen(true);
  };

  const request: SessionContinueRequest = source => openTarget("continue", source);
  const requestFork: SessionForkRequest = (source, point) => openTarget("fork", source, point);

  return { workspaceId, target, open, setOpen, request, requestFork };
}

export type SessionContinueHostState = ReturnType<typeof useSessionContinueHost>;
