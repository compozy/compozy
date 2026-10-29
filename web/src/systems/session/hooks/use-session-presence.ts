import { useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { sessionKeys } from "../lib/query-keys";
import { useSessionDeletionPending, useSessionDeletedByOperator } from "./use-session-actions";
import { isSessionUnavailable } from "../adapters/session-api-errors";

import {
  acquireSessionPresence,
  releaseSessionPresence,
  renewSessionPresence,
} from "../adapters/session-presence-api";

/** Lease TTL is 15s daemon-side; renewing at 5s survives two missed ticks. */
const PRESENCE_RENEW_INTERVAL_MS = 5_000;

function releasePresence(workspaceId: string, sessionId: string, leaseId: string): void {
  void releaseSessionPresence(workspaceId, sessionId, leaseId).catch(() => {
    // Release is best-effort. The daemon expires a missed release at the
    // 15-second lease TTL, so there is no truthful operator action to offer.
  });
}

function startPresenceLoop(
  workspaceId: string,
  sessionId: string,
  shouldRelease: () => boolean
): () => void {
  let leaseId: string | null = null;
  let cancelled = false;
  let timer: ReturnType<typeof setTimeout> | null = null;
  const controller = new AbortController();

  const schedule = (cycle: () => Promise<void>) => {
    if (cancelled) return;
    timer = setTimeout(() => {
      timer = null;
      void cycle();
    }, PRESENCE_RENEW_INTERVAL_MS);
  };

  const cycle = async (): Promise<void> => {
    if (cancelled) return;
    if (leaseId === null) {
      try {
        const acquired = await acquireSessionPresence(workspaceId, sessionId, controller.signal);
        if (cancelled) {
          if (shouldRelease()) releasePresence(workspaceId, sessionId, acquired);
          return;
        }
        leaseId = acquired;
      } catch (error) {
        if (isSessionUnavailable(error)) return;
        // Presence is best-effort. Failure leaves the session eligible for
        // `done`, which is the honest daemon-side outcome.
      }
    } else {
      const renewal = await renewSessionPresence(
        workspaceId,
        sessionId,
        leaseId,
        controller.signal
      );
      if (cancelled) return;
      // A daemon restart or TTL expiry invalidates the lease. The next cycle
      // acquires a new one instead of retrying a dead id. Transient failures
      // retain the lease and retry on the bounded five-second loop.
      if (renewal === "lease-invalid") leaseId = null;
    }
    schedule(cycle);
  };

  void cycle();

  return () => {
    cancelled = true;
    controller.abort();
    if (timer !== null) clearTimeout(timer);
    if (leaseId !== null && shouldRelease()) releasePresence(workspaceId, sessionId, leaseId);
    leaseId = null;
  };
}

/**
 * Reports that the operator is watching this session.
 *
 * This is the only thing that clears a `done` marker. Requests run in one
 * completion-driven chain so a slow acquire or renew can never create another
 * lease concurrently. Mount this only in the focused, visible session window;
 * passive attention surfaces never report presence.
 */
export function useSessionPresence(
  workspaceId: string | null,
  sessionId: string | null,
  watching: boolean
): void {
  const deletionPending = useSessionDeletionPending(sessionId);
  const deleted = useSessionDeletedByOperator(sessionId);
  const available = !deletionPending && !deleted;
  const queryClient = useQueryClient();
  useEffect(() => {
    if (!watching || !available || !workspaceId || !sessionId) return undefined;
    // Read the captured session's live mutation state, including late acquire cleanup.
    const shouldRelease = () =>
      !queryClient.getMutationCache().find({
        mutationKey: sessionKeys.deletions(),
        predicate: mutation => {
          const variables = mutation.state.variables;
          return (
            (mutation.state.status === "pending" || mutation.state.status === "success") &&
            typeof variables === "object" &&
            variables !== null &&
            "id" in variables &&
            variables.id === sessionId
          );
        },
      });
    if (!shouldRelease()) return undefined;
    return startPresenceLoop(workspaceId, sessionId, shouldRelease);
  }, [workspaceId, sessionId, watching, available, queryClient]);
}
