import { createStoreLogic } from "@xstate/store";
import { useSelector } from "@xstate/store-react";
import { useEffect, useEffectEvent } from "react";

import type { SessionLifecycleActionHandlers, SessionPayload } from "@/systems/session";

/** One delete the window rail asks the shell to confirm. */
export interface SessionDeleteRequest {
  sessions: readonly SessionPayload[];
  bulk: boolean;
  onSelectionChange?: (remainingIds: readonly string[]) => void;
}

interface SessionDeleteRequestContext {
  request: SessionDeleteRequest | null;
}

const sessionDeleteRequestLogic = createStoreLogic({
  context: (): SessionDeleteRequestContext => ({ request: null }),
  on: {
    requested: (_context, event: { request: SessionDeleteRequest }) => ({
      request: event.request,
    }),
    settled: () => ({ request: null }),
  },
});

export const sessionDeleteRequestStore = sessionDeleteRequestLogic.createStore();

/**
 * Offers a delete confirmation to the shell host, which outlives every session
 * window: the daemon reconciles a deleted session's window before DELETE returns.
 */
export function requestSessionDelete(request: SessionDeleteRequest): void {
  sessionDeleteRequestStore.trigger.requested({ request });
}

export function settleSessionDeleteRequest(): void {
  sessionDeleteRequestStore.trigger.settled();
}

/** The pending delete request, or `null`. */
export function useSessionDeleteRequest(): SessionDeleteRequest | null {
  return useSelector(sessionDeleteRequestStore, snapshot => snapshot.context.request);
}

/**
 * Hosts the rail's delete confirmations at the shell. The request is consumed
 * once and cleared, so a re-render cannot reopen the same confirmation.
 */
export function useHostedSessionDeleteRequests(actions: SessionLifecycleActionHandlers): void {
  const handleRequest = useEffectEvent((pending: SessionDeleteRequest) => {
    if (pending.bulk) {
      actions.onDeleteMany?.(pending.sessions, pending.onSelectionChange);
      return;
    }
    const session = pending.sessions[0];
    if (session) actions.onDelete(session);
  });
  useEffect(() => {
    const subscription = sessionDeleteRequestStore.subscribe(snapshot => {
      const pending = snapshot.context.request;
      if (!pending) return;
      handleRequest(pending);
      settleSessionDeleteRequest();
    });
    return () => subscription.unsubscribe();
  }, []);
}
