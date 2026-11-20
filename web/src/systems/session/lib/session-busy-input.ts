import type { SessionPayload } from "../types";

/** The operator's follow-up default for a send during an active turn (ADR-002). */
export type SessionBusyInputMode = "steer" | "queue";

/** Every explicit busy-send verb the composer can issue; explicit verbs always win over the default. */
export type SessionBusyInputAction = SessionBusyInputMode | "interrupt";

/** Any send the client retains an identity for: a busy verb, or the idle prompt that starts a turn. */
export type SessionSendAction = SessionBusyInputAction | "prompt";

/** Public steer delivery vocabulary — exactly the daemon's `SteerDeliveryMode`. */
export type SessionSteerDelivery =
  | "injected"
  | "pending_injection"
  | "pending_boundary"
  | "after_turn"
  | "interrupt_fallback";

/** Daemon default when the session resource has not reported one yet. */
export const DEFAULT_SESSION_BUSY_INPUT_MODE: SessionBusyInputMode = "steer";

type SessionWithBusyInput = Pick<SessionPayload, "busy_input">;

function isBusyInputMode(value: string | undefined): value is SessionBusyInputMode {
  return value === "steer" || value === "queue";
}

/** The daemon-owned follow-up default this session resolves for unmarked busy sends. */
export function sessionBusyInputDefaultMode(session: SessionWithBusyInput): SessionBusyInputMode {
  const mode = session.busy_input?.default_mode?.trim();
  return isBusyInputMode(mode) ? mode : DEFAULT_SESSION_BUSY_INPUT_MODE;
}

/** The one-shot opposite the modifier applies for a single send (US-003.AC-3). */
export function oppositeSessionBusyInputMode(mode: SessionBusyInputMode): SessionBusyInputMode {
  return mode === "steer" ? "queue" : "steer";
}
