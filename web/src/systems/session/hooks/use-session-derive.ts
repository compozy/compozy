import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import {
  continueSession,
  type ContinueSessionRequest,
  type SessionDeriveResult,
} from "../adapters/session-derive-api";
import { SessionApiError } from "../adapters/session-api-errors";
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
  if (placement === "this-window" && handlers.openInThisWindow) {
    handlers.openInThisWindow(child);
  } else {
    handlers.openInNewWindow(child);
  }
  return "opened";
}

export interface UseSessionDerivePreviewOptions {
  enabled: boolean;
  messageId?: string;
}

/** The dialog's context line: measured on open, projected to its view model. */
export function useSessionDerivePreview(
  workspaceId: string,
  sessionId: string,
  { enabled, messageId }: UseSessionDerivePreviewOptions
) {
  const options = sessionDerivePreviewOptions(workspaceId, sessionId, messageId);
  const query = useQuery({ ...options, enabled: enabled && options.enabled === true });
  return {
    preview: query.data,
    view: sessionDerivePreviewView(query.data, query.status),
  };
}

export interface SessionContinueVariables {
  workspaceId: string;
  sourceId: string;
  request: ContinueSessionRequest;
}

/**
 * Continues a source session. The source's by-id reads and the catalog are
 * reread (the child nests under it); the child's detail is seeded from the
 * authoritative response so its window opens without a second round trip.
 */
export function useSessionContinue() {
  const queryClient = useQueryClient();
  return useMutation<SessionDeriveResult, Error, SessionContinueVariables>({
    mutationFn: ({ workspaceId, sourceId, request }) =>
      continueSession(workspaceId, sourceId, request),
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

function newIdempotencyKey(): string {
  return `web-derive-${globalThis.crypto.randomUUID()}`;
}

/**
 * One idempotency key per dialog open, so a retry after a lost response
 * replays instead of creating a second child. A daemon refusal (4xx) created
 * nothing, so the next attempt — possibly a different request — takes a new
 * key instead of tripping `idempotency_conflict`.
 */
export function useSessionDeriveIdempotencyKey() {
  const [key, setKey] = useState(newIdempotencyKey);
  return {
    key,
    settleFailure: (error: unknown) => {
      if (error instanceof SessionApiError && error.status >= 400 && error.status < 500) {
        setKey(newIdempotencyKey());
      }
    },
  };
}
