import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";

import { readProfileScopeParams } from "@/systems/profiles";

import {
  continueSession,
  type ContinueSessionRequest,
  forkSession,
  type ForkSessionRequest,
  type SessionDeriveResult,
} from "../adapters/session-derive-api";
import { sessionKeys } from "../lib/query-keys";
import { sessionScopedDetailOptions } from "../lib/query-options";
import {
  isSessionDeriveAbort,
  isSessionDeriveTimeout,
  SESSION_DERIVE_SUBMIT_TIMEOUT_MS,
  sessionDeriveDeadline,
} from "../lib/session-derive-deadline";
import { sessionDerivePreviewOptions } from "../lib/session-derive-query";
import { sessionDerivePreviewView } from "../lib/session-derive-view";
import type { SessionPayload } from "../types";

/**
 * Where a derived child opens. Continue offers both (`new-window` keeps the
 * source visible and is its default); Fork always lands `this-window`.
 */
export type SessionDerivePlacement = "new-window" | "this-window";

/**
 * Host-owned landings. New window is the attention-jump `userOpen` path, which
 * focuses a window the child already owns instead of opening a second one;
 * This window is the host's in-place retarget. A host without a session window
 * of its own omits `openInThisWindow`: Continue does not offer the choice and
 * every child, a fork's included, opens in its own window.
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
 * A failed or timed-out measurement offers `retry`, which reads measuring again
 * while it runs; `failed` tells the dialog it may submit without the measurement.
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
  const status = measuredThisOpen && !query.isFetching ? query.status : "pending";
  const view = sessionDerivePreviewView(preview, status, isSessionDeriveTimeout(query.error));
  return {
    preview,
    view,
    failed: view.state === "error",
    retry: () => {
      void query.refetch();
    },
  };
}

export interface SessionDeriveVariables<Request> {
  workspaceId: string;
  sourceId: string;
  request: Request;
  /** Aborts the request: the operator left, or the submit deadline passed. */
  signal?: AbortSignal;
}

export type SessionContinueVariables = SessionDeriveVariables<ContinueSessionRequest>;
export type SessionForkVariables = SessionDeriveVariables<ForkSessionRequest>;

/**
 * One derive verb over its route. The source's by-id reads and the catalog are
 * reread (the child nests under it). The child is seeded from the authoritative
 * response into the reads its window resolves through (the profile-enforced
 * by-id lens of the current view and of the child's own profile, plus the
 * workspace detail), so the window opens on it without a second round trip.
 * A request the operator left or that timed out may still have created the
 * child, so the catalog is reread then too.
 */
function useSessionDeriveMutation<Request>(
  derive: (
    workspaceId: string,
    sourceId: string,
    request: Request,
    signal?: AbortSignal
  ) => Promise<SessionDeriveResult>
) {
  const queryClient = useQueryClient();
  return useMutation<SessionDeriveResult, Error, SessionDeriveVariables<Request>>({
    mutationFn: ({ workspaceId, sourceId, request, signal }) =>
      derive(workspaceId, sourceId, request, signal),
    onSuccess: (result, { workspaceId, sourceId }) => {
      void Promise.all([
        queryClient.invalidateQueries({ queryKey: sessionKeys.lists() }),
        queryClient.invalidateQueries({ queryKey: sessionKeys.byIdRoot(sourceId) }),
        queryClient.invalidateQueries({
          queryKey: sessionKeys.byIdRoot(result.derived.child_session_id),
        }),
      ]);
      // After the invalidation: the seeded entries are the fresh answer, not stale ones.
      const child = result.session;
      if (child) {
        queryClient.setQueryData(sessionKeys.detail(workspaceId, child.id), child);
        const owner = child.profile_name?.trim() ?? "";
        const scopes = [
          readProfileScopeParams(queryClient),
          ...(owner ? [{ profile: owner }] : []),
        ];
        for (const scope of scopes) {
          queryClient.setQueryData(sessionScopedDetailOptions(child.id, scope).queryKey, child);
        }
      }
    },
    onError: error => {
      if (isSessionDeriveAbort(error) || isSessionDeriveTimeout(error)) {
        void queryClient.invalidateQueries({ queryKey: sessionKeys.lists() });
      }
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

/**
 * The submit's abort handle. Each submit gets a fresh signal bounded by the
 * submit deadline; `cancel` stops waiting when the operator leaves, and an
 * unmounted dialog stops waiting too. The daemon's work is not undone: a child
 * it already created stays reachable through the same idempotency key.
 */
export function useSessionDeriveAbort() {
  const controllerRef = useRef<AbortController | null>(null);
  useEffect(() => () => controllerRef.current?.abort(), []);
  return {
    next: (): AbortSignal => {
      controllerRef.current?.abort();
      const controller = new AbortController();
      controllerRef.current = controller;
      return sessionDeriveDeadline(SESSION_DERIVE_SUBMIT_TIMEOUT_MS, controller.signal);
    },
    cancel: () => {
      controllerRef.current?.abort();
      controllerRef.current = null;
    },
  };
}

/** The submit outlived its deadline: Try again replays the same key and opens a child it created. */
export const SESSION_DERIVE_TIMED_OUT =
  "Starting the new session took too long. Try again; if it was created, it opens.";

/** The words a failed submit shows; `null` when the operator left and nothing is shown. */
export function sessionDeriveFailureText(error: Error): string | null {
  if (isSessionDeriveAbort(error)) return null;
  if (isSessionDeriveTimeout(error)) return SESSION_DERIVE_TIMED_OUT;
  return error.message;
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
