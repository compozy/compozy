import { useEffect, useEffectEvent } from "react";

import type { SessionLifecycleActionHandlers } from "@/systems/session";

import {
  subscribeSessionDeleteRequests,
  type SessionDeleteRequest,
} from "../lib/session-delete-request";

/**
 * Hosts the window rail's delete confirmations at the shell, which outlives the session window
 * the daemon reconciles before the DELETE answers. The effect event keeps the subscription stable
 * while the lifecycle owner re-renders.
 */
export function useHostedSessionDeleteRequests(actions: SessionLifecycleActionHandlers): void {
  const handleRequest = useEffectEvent((request: SessionDeleteRequest) => {
    if (request.bulk) {
      actions.onDeleteMany?.(request.sessions, request.onSelectionChange);
      return;
    }
    const session = request.sessions[0];
    if (session) actions.onDelete(session);
  });
  useEffect(() => subscribeSessionDeleteRequests(handleRequest), []);
}
