import { useEffect } from "react";

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

function startPresenceLoop(workspaceId: string, sessionId: string): () => void {
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
          releasePresence(workspaceId, sessionId, acquired);
          return;
        }
        leaseId = acquired;
      } catch {
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
    if (leaseId !== null) releasePresence(workspaceId, sessionId, leaseId);
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
  useEffect(() => {
    if (!watching || !workspaceId || !sessionId) return undefined;
    return startPresenceLoop(workspaceId, sessionId);
  }, [workspaceId, sessionId, watching]);
}
