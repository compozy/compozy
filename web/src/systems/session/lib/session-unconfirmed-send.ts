import { SessionApiError } from "../adapters/session-api-errors";
import type { SessionPromptRuntimeSnapshot } from "../contexts/session-prompt-runtime-context-value";
import type { SessionPromptAttachment } from "../types";
import type { SessionSendAction } from "./session-busy-input";

/** The send identity the daemon admits exactly once (invariant 8). */
export interface SessionSendIdentity {
  idempotencyKey: string;
  messageId: string;
}

/** Everything a busy send carried, kept so a replay reproduces it byte for byte. */
export interface SessionSendEnvelope {
  action: SessionSendAction;
  attachments: SessionPromptAttachment[];
  expectedTurnId: string | null;
  identity: SessionSendIdentity;
  runtime: SessionPromptRuntimeSnapshot | null;
  text: string;
}

/**
 * A send whose acknowledgment never arrived — client-local presentation only
 * (Part II Key Decision: `unconfirmed` has exactly one owner, the client).
 * `unconfirmed` waits for the operator; `retrying` is the replay in flight,
 * still pending its acknowledgment.
 */
export interface UnconfirmedSend extends SessionSendEnvelope {
  /** The message id doubles as the row key: one identity, one row. */
  id: string;
  phase: "unconfirmed" | "retrying";
}

/**
 * Whether a failed send left the daemon's admission unknown. Only a daemon
 * answer in the 4xx range is an authoritative outcome (refused, conflicting,
 * not found, not allowed): nothing was recorded. Everything else — a network
 * failure, a mid-request disconnect, an abort that fired after the bytes left,
 * a gateway timeout, or a 5xx thrown after durable acceptance — says nothing
 * about whether the daemon recorded the send, so the identity is retained and
 * only a replay of that identity can tell.
 */
export function isSendAcknowledgmentLost(error: unknown): boolean {
  if (error instanceof SessionApiError) {
    return !(error.status >= 400 && error.status < 500);
  }
  return typeof error === "object" && error !== null;
}

/** Marks a send unconfirmed; a retry that failed the same way returns to waiting. */
export function markSendUnconfirmed(
  sends: readonly UnconfirmedSend[],
  envelope: SessionSendEnvelope
): UnconfirmedSend[] {
  const id = envelope.identity.messageId;
  const next: UnconfirmedSend = { ...envelope, id, phase: "unconfirmed" };
  if (sends.some(send => send.id === id)) {
    return sends.map(send => (send.id === id ? next : send));
  }
  return [...sends, next];
}

/** The replay left with the retained identity; the row keeps its preview while it waits. */
export function beginUnconfirmedRetry(
  sends: readonly UnconfirmedSend[],
  id: string
): UnconfirmedSend[] {
  return sends.map(send =>
    send.id === id && send.phase === "unconfirmed" ? { ...send, phase: "retrying" } : send
  );
}

/**
 * An observed outcome — replayed, fresh, or refused — resolves the marker; so
 * does an explicit discard. Either way the client stops holding the identity.
 */
export function resolveUnconfirmedSend(
  sends: readonly UnconfirmedSend[],
  id: string
): UnconfirmedSend[] {
  return sends.filter(send => send.id !== id);
}

export function findUnconfirmedSend(
  sends: readonly UnconfirmedSend[],
  id: string
): UnconfirmedSend | null {
  return sends.find(send => send.id === id) ?? null;
}
