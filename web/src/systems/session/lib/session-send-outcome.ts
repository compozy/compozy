import type { SessionSteerDelivery } from "./session-busy-input";
import type { SessionPromptSendResult } from "../types";

export type SessionSendDisposition = "direct" | "steering" | "queued" | "interrupting";

/**
 * Flattened `SendOutcome` envelope of a busy send — what the composer answers
 * with inline (US-004.AC-1). `steerDelivery` is set only for `steering`;
 * `queuePosition` only for `queued`.
 */
export interface SessionSendOutcome {
  disposition: SessionSendDisposition;
  steerDelivery: SessionSteerDelivery | null;
  turnId: string | null;
  entryId: string | null;
  messageId: string | null;
  idempotencyKey: string | null;
  queuePosition: number | null;
  replayed: boolean;
}

function legacyDisposition(status: string): SessionSendDisposition | null {
  switch (status.trim()) {
    case "queued":
      return "queued";
    case "steering":
      return "steering";
    case "interrupting":
      return "interrupting";
    case "accepted":
      return "direct";
    default:
      return null;
  }
}

function isSteerDelivery(value: unknown): value is SessionSteerDelivery {
  return value === "injected" || value === "pending_injection" || value === "interrupt_fallback";
}

function nonEmpty(value: string | undefined): string | null {
  const normalized = value?.trim() ?? "";
  return normalized.length > 0 ? normalized : null;
}

/**
 * Reads the disposition envelope from a busy-send result. Goal command results
 * carry no envelope and yield `null`; a direct turn that streamed instead of
 * answering with JSON reports `direct` (the turn ended before admission).
 */
export function sessionSendOutcomeFromResult(
  result: SessionPromptSendResult
): SessionSendOutcome | null {
  if ("direct_turn" in result) {
    return {
      disposition: "direct",
      entryId: null,
      idempotencyKey: result.idempotency_key,
      messageId: result.message_id,
      queuePosition: null,
      replayed: false,
      steerDelivery: null,
      turnId: null,
    };
  }
  if (!("status" in result)) {
    return null;
  }
  const disposition = result.disposition ?? legacyDisposition(result.status);
  if (disposition === null) {
    return null;
  }
  const steerDelivery =
    disposition === "steering" && isSteerDelivery(result.steer_delivery)
      ? result.steer_delivery
      : null;
  const queuePosition =
    disposition === "queued" &&
    typeof result.queue_position === "number" &&
    result.queue_position > 0
      ? result.queue_position
      : null;
  return {
    disposition,
    entryId: nonEmpty(result.entry_id) ?? nonEmpty(result.queue_entry_id),
    idempotencyKey: nonEmpty(result.idempotency_key),
    messageId: nonEmpty(result.message_id),
    queuePosition,
    replayed: result.replayed,
    steerDelivery,
    turnId: nonEmpty(result.turn_id) ?? nonEmpty(result.new_turn_id),
  };
}
