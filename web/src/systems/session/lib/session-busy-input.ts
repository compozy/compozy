import type { SessionPayload } from "../types";

/** The operator's follow-up default for a send during an active turn (ADR-002). */
export type SessionBusyInputMode = "steer" | "queue";

/** Every explicit busy-send verb the composer can issue; explicit verbs always win over the default. */
export type SessionBusyInputAction = SessionBusyInputMode | "interrupt";

/** Public steer delivery vocabulary — exactly the daemon's `SteerDeliveryMode`. */
export type SessionSteerDelivery = "injected" | "pending_injection" | "interrupt_fallback";

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

/**
 * How a steer would land on this session's agent, answered before any send
 * (US-002.AC-3). The prediction derives from the resolved capability
 * (ADR-010): `steer_ext` injects live, `concurrent_prompt` is accepted and
 * lands when the current tool yields, `none` interrupts and replaces. The
 * capability always wins over `steer_delivery`, which only records the last
 * send and may describe a provider that has since changed; an unknown
 * capability answers `null` rather than guessing.
 */
export function sessionSteerDelivery(session: SessionWithBusyInput): SessionSteerDelivery | null {
  switch (session.busy_input?.steer_capability?.trim()) {
    case "steer_ext":
      return "injected";
    case "concurrent_prompt":
      return "pending_injection";
    case "none":
      return "interrupt_fallback";
    default:
      return null;
  }
}

/** The one-shot opposite the modifier applies for a single send (US-003.AC-3). */
export function oppositeSessionBusyInputMode(mode: SessionBusyInputMode): SessionBusyInputMode {
  return mode === "steer" ? "queue" : "steer";
}
