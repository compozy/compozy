import { useState } from "react";

import type { SessionContinueRequest } from "../contexts/session-derive-context-value";
import type { SessionForkPoint, SessionForkRequest } from "../contexts/session-fork-context-value";
import type { SessionProfileMoveTarget } from "../contexts/session-profile-move-context-value";
import { isUserControllableSession } from "../lib/session-running";
import type { SessionPayload } from "../types";

interface DeriveTarget {
  /** Which dialog the host mounts: Continue (another agent), Fork (same agent), or Move to profile. */
  kind: "continue" | "fork" | "profile-move";
  source: SessionPayload;
  /** Fork from here: the message the fork cuts through; absent for the whole session. */
  point?: SessionForkPoint;
  /** Move to profile: the profile the copy starts in. */
  profile?: SessionProfileMoveTarget;
  /** Each open starts fresh: a new measurement and a new idempotency key. */
  nonce: number;
}

export interface UseSessionDeriveHostInput {
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
export function useSessionDeriveHost({ workspaceId, currentSession }: UseSessionDeriveHostInput) {
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
  const requestProfileMove = (source: SessionPayload, profile: SessionProfileMoveTarget) => {
    if (!isUserControllableSession(source)) return;
    setTarget(previous => ({
      kind: "profile-move",
      source,
      profile,
      nonce: (previous?.nonce ?? 0) + 1,
    }));
    setOpen(true);
  };

  return { workspaceId, target, open, setOpen, request, requestFork, requestProfileMove };
}

export type SessionDeriveHostState = ReturnType<typeof useSessionDeriveHost>;
