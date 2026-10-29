import { use } from "react";

import { SessionForkContext } from "../contexts/session-fork-context-value";
import { useSessionMessageActionGate } from "./use-session-message-action-gate";

/**
 * "Fork from here" on one user message: available exactly where "Rewind to
 * here" is (a durable user message) and only under a host that can open the
 * Fork dialog; busy under the same gates.
 */
export function useSessionForkMessageAction() {
  const gate = useSessionMessageActionGate();
  const requestFork = use(SessionForkContext);
  const available = gate.durable && requestFork !== null;

  /** Opens the Fork dialog through this message; a no-op while unavailable or busy. */
  const trigger = () => {
    if (!available || gate.busy) return;
    requestFork(undefined, { messageId: gate.messageId });
  };

  return { available, busy: gate.busy, trigger };
}
