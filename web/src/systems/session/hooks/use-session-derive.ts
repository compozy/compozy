import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import {
  continueSession,
  type ContinueSessionRequest,
  forkSession,
  type ForkSessionRequest,
  type SessionDeriveResult,
} from "../adapters/session-derive-api";
import { sessionKeys } from "../lib/query-keys";
import { sessionDerivePreviewOptions } from "../lib/session-derive-query";
import { sessionDerivePreviewView } from "../lib/session-derive-view";
import type { SessionPayload } from "../types";

/** Where a derived child opens. `new-window` keeps the source visible (the default). */
export type SessionDerivePlacement = "new-window" | "this-window";

/**
 * Host-owned landings. New window is the attention-jump `userOpen` path, which
 * focuses a window the child already owns instead of opening a second one;
 * This window is the host's in-place retarget. A host without a session window
 * of its own omits `openInThisWindow`, and the choice is not offered.
 */
export interface SessionDerivePlacementHandlers {
  openInNewWindow: (child: SessionPayload) => void;
  openInThisWindow?: (child: SessionPayload) => void;
}

export type SessionDeriveLanding = "opened" | "child_deleted";

/**
 * Lands one derive outcome exactly once: a new child and a replayed one
 * (`replayed: true`, the same key already created it) both open that child;
 * `child_deleted` opens nothing.
 */
export function landDerivedSession(
  result: SessionDeriveResult,
  placement: SessionDerivePlacement,
  handlers: SessionDerivePlacementHandlers
): SessionDeriveLanding {
  const child = result.session;
  if (result.derived.child_deleted || !child) return "child_deleted";
  openDerivedChild(child, placement, handlers);
  return "opened";
}

/** Opens one derived child where the operator chose (This window only when the host has one). */
export function openDerivedChild(
  child: SessionPayload,
  placement: SessionDerivePlacement,
  handlers: SessionDerivePlacementHandlers
): void {
  if (placement === "this-window" && handlers.openInThisWindow) {
    handlers.openInThisWindow(child);
  } else {
    handlers.openInNewWindow(child);
  }
}

export interface UseSessionDerivePreviewOptions {
  enabled: boolean;
  messageId?: string;
}

/**
 * The dialog's context line: measured on open, projected to its view model.
 * Each open mounts a fresh dialog (the host's nonce), so only an answer fetched
 * after this mount counts. A previous open's cached answer can outlive `gcTime: 0`
 * while the next open remeasures, and it would show an old size and hand Fork
 * obsolete fences; until this open's own answer arrives the line reads measuring.
 */
export function useSessionDerivePreview(
  workspaceId: string,
  sessionId: string,
  { enabled, messageId }: UseSessionDerivePreviewOptions
) {
  const options = sessionDerivePreviewOptions(workspaceId, sessionId, messageId);
  const query = useQuery({ ...options, enabled: enabled && options.enabled === true });
  const measuredThisOpen = query.isFetchedAfterMount;
  const preview = measuredThisOpen ? query.data : undefined;
  return {
    preview,
    view: sessionDerivePreviewView(preview, measuredThisOpen ? query.status : "pending"),
  };
}

export interface SessionDeriveVariables<Request> {
  workspaceId: string;
  sourceId: string;
  request: Request;
}

export type SessionContinueVariables = SessionDeriveVariables<ContinueSessionRequest>;
export type SessionForkVariables = SessionDeriveVariables<ForkSessionRequest>;

/**
 * One derive verb over its route. The source's by-id reads and the catalog are
 * reread (the child nests under it); the child's detail is seeded from the
 * authoritative response so its window opens without a second round trip.
 */
function useSessionDeriveMutation<Request>(
  derive: (workspaceId: string, sourceId: string, request: Request) => Promise<SessionDeriveResult>
) {
  const queryClient = useQueryClient();
  return useMutation<SessionDeriveResult, Error, SessionDeriveVariables<Request>>({
    mutationFn: ({ workspaceId, sourceId, request }) => derive(workspaceId, sourceId, request),
    onSuccess: (result, { workspaceId, sourceId }) => {
      const child = result.session;
      if (child) {
        queryClient.setQueryData(sessionKeys.detail(workspaceId, child.id), child);
      }
      void Promise.all([
        queryClient.invalidateQueries({ queryKey: sessionKeys.lists() }),
        queryClient.invalidateQueries({ queryKey: sessionKeys.byIdRoot(sourceId) }),
        queryClient.invalidateQueries({
          queryKey: sessionKeys.byIdRoot(result.derived.child_session_id),
        }),
      ]);
    },
  });
}

/** Continues a source session with another agent. */
export function useSessionContinue() {
  return useSessionDeriveMutation<ContinueSessionRequest>(continueSession);
}

/** Forks a source session: same agent, whole or through one message's turn. */
export function useSessionFork() {
  return useSessionDeriveMutation<ForkSessionRequest>(forkSession);
}

function newIdempotencyKey(): string {
  return `web-derive-${globalThis.crypto.randomUUID()}`;
}

/**
 * One idempotency key per dialog open, kept across failures: a retry after a
 * lost response replays, and a retry after a failure that followed the commit
 * (the first message's admission refused after the child exists) completes that
 * same child instead of creating a second one. A daemon refusal before the
 * commit records no receipt, so reusing the key for a changed request is safe.
 */
export function useSessionDeriveIdempotencyKey(): string {
  const [key] = useState(newIdempotencyKey);
  return key;
}
