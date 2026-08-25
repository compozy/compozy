/**
 * Reading a daemon-authored turn out of a transcript message.
 *
 * Calls and messages are not spliced into the transcript by the browser — they
 * *are* transcript turns. The daemon writes them as synthetic prompts and
 * projects a bounded, storage-opaque descriptor onto the message's metadata
 * (`internal/transcript/ui_input_messages.go`). So the order on screen is the
 * durable order, and this module's only job is to recognize which kind of turn a
 * message is.
 *
 * That is why there is no timestamp merge anywhere in this system: interleaving
 * two sources by clock would invent an ordering the runtime never recorded, and
 * would disagree with the child's own context.
 *
 * The metadata is deliberately narrow — no result references, no claim hashes,
 * no policy internals — so everything here is safe to render.
 */

/** What a synthetic turn is for. Anything else is not one of ours. */
export type SyntheticTurnKind =
  /** The initial ask that started a call. */
  | "call-request"
  /** A follow-up ask that revived an existing child. */
  | "call-follow-up"
  /** A call settled and woke its caller, carrying the outcome. */
  | "call-wake"
  /** A message delivered into this session's turn. */
  | "message";

export interface SyntheticTurn {
  kind: SyntheticTurnKind;
  callId: string | null;
  /** The call's state at the moment this turn was written. */
  callState: string | null;
  childSessionId: string | null;
  childAgentName: string | null;
  resultBytes: number | null;
  contractDigest: string | null;
  messageId: string | null;
  /** Public delivery receipt for a message turn. */
  deliveryKind: string | null;
  /** The daemon's own reason token. */
  reason: string | null;
  /** The daemon's own summary line — rendered verbatim, never rephrased. */
  summary: string | null;
  wakeEventId: string | null;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function text(source: Record<string, unknown>, key: string): string | null {
  const value = source[key];
  return typeof value === "string" && value.trim() !== "" ? value : null;
}

function count(source: Record<string, unknown>, key: string): number | null {
  const value = source[key];
  return typeof value === "number" && Number.isFinite(value) ? value : null;
}

/**
 * Which kind of turn this is.
 *
 * The daemon's `reason` decides it where it says so; otherwise the shape does.
 * A descriptor with a message id and no call is a mailbox delivery; one with a
 * call id and a settled state is the completion wake. Anything that names
 * neither is not a synthetic call turn and is left as an ordinary message.
 */
function classify(source: Record<string, unknown>): SyntheticTurnKind | null {
  const reason = text(source, "reason");
  if (reason === "call_request") return "call-request";
  if (reason === "call_follow_up") return "call-follow-up";

  const messageId = text(source, "message_id");
  const callId = text(source, "call_id");
  if (messageId !== null && callId === null) return "message";
  if (messageId !== null && text(source, "delivery_kind") !== null) return "message";
  if (callId !== null) return "call-wake";
  return null;
}

/**
 * Parse the synthetic descriptor off a transcript message's metadata.
 *
 * Returns null for an ordinary operator or agent turn, which is the common case
 * — every message in the transcript passes through here.
 */
export function readSyntheticTurn(metadata: unknown): SyntheticTurn | null {
  // Assistant-ui nests caller metadata under `custom` on some transports; the
  // skill-invocation reader handles the same two shapes.
  const root = isRecord(metadata) && isRecord(metadata.custom) ? metadata.custom : metadata;
  if (!isRecord(root)) return null;
  const source = root.synthetic;
  if (!isRecord(source)) return null;

  const kind = classify(source);
  if (kind === null) return null;

  return {
    kind,
    callId: text(source, "call_id"),
    callState: text(source, "call_state"),
    childSessionId: text(source, "child_session_id"),
    childAgentName: text(source, "child_agent_name"),
    resultBytes: count(source, "result_bytes"),
    contractDigest: text(source, "contract_digest"),
    messageId: text(source, "message_id"),
    deliveryKind: text(source, "delivery_kind"),
    reason: text(source, "reason"),
    summary: text(source, "summary"),
    wakeEventId: text(source, "wake_event_id"),
  };
}
