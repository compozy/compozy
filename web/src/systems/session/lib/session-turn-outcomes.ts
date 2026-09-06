// Turn outcomes across the loaded thread (task_07 / US-022): the daemon
// records how a turn ended on events that may land in a later projected
// message than the turn's tool calls — the durable `session.turn_quiesced`
// receipt, the operator's `prompt_cancel` / `prompt_interrupted` markers, a
// `session_stopped` event. A message renders its own parts, so this index says,
// per turn id, whether the turn already ended: a call still awaiting its result
// in an ended turn is stopped, never "running". Pure over the loaded messages;
// no timestamps are fabricated — `endedAtMs` is the recording event's own.

import { isAgentEventPayload } from "./message-parts";

export type SessionTurnOutcomeKind = "interrupted" | "stopped";

export interface SessionTurnOutcome {
  turnId: string;
  kind: SessionTurnOutcomeKind;
  /** When the daemon recorded the end; `null` when the event carried no timestamp. */
  endedAtMs: number | null;
  /** The durable turn receipt said the stop was verified (turn scope). */
  verified: boolean;
}

export type SessionTurnOutcomes = ReadonlyMap<string, SessionTurnOutcome>;

const EMPTY: SessionTurnOutcomes = new Map();

export function emptyTurnOutcomes(): SessionTurnOutcomes {
  return EMPTY;
}

interface ThreadMessageLike {
  role?: string;
  content?: unknown;
}

const TURN_QUIESCED_EVENT = "session.turn_quiesced";
const SESSION_STOPPED_EVENT = "session_stopped";
const INTERRUPT_MARKERS = new Set([
  "transcript_marker.prompt_cancel",
  "transcript_marker.prompt_interrupted",
]);

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function stringField(record: Record<string, unknown>, key: string): string | undefined {
  const value = record[key];
  return typeof value === "string" && value.length > 0 ? value : undefined;
}

function instantMs(value: string | undefined): number | null {
  if (!value) return null;
  const parsed = Date.parse(value);
  return Number.isFinite(parsed) ? parsed : null;
}

function outcomeOf(data: Record<string, unknown>): SessionTurnOutcome | null {
  if (!isAgentEventPayload(data)) return null;
  const type = data.type;
  const endedAtMs = instantMs(stringField(data, "timestamp"));
  if (type === TURN_QUIESCED_EVENT) {
    const raw = data.raw;
    if (!isRecord(raw) || raw.scope !== "turn") return null;
    const turnId = stringField(raw, "turn_id") ?? stringField(data, "turn_id");
    if (!turnId) return null;
    return { turnId, kind: "interrupted", endedAtMs, verified: raw.verified === true };
  }
  const turnId = stringField(data, "turn_id");
  if (!turnId) return null;
  if (type === SESSION_STOPPED_EVENT) {
    return { turnId, kind: "stopped", endedAtMs, verified: false };
  }
  const markerKind = data.marker?.kind;
  if (markerKind && INTERRUPT_MARKERS.has(markerKind)) {
    return { turnId, kind: "interrupted", endedAtMs, verified: false };
  }
  return null;
}

/**
 * Per turn id, the daemon's record that the turn ended, wherever in the loaded
 * thread it was projected. A verified receipt outranks a marker; otherwise the
 * latest recording wins.
 */
export function deriveTurnOutcomes(messages: readonly ThreadMessageLike[]): SessionTurnOutcomes {
  const outcomes = new Map<string, SessionTurnOutcome>();
  for (const message of messages) {
    if (message.role !== "assistant" || !Array.isArray(message.content)) continue;
    for (const part of message.content) {
      if (!isRecord(part)) continue;
      const type = part.type;
      const isEvent =
        type === "data-compozy-event" || (type === "data" && part.name === "compozy-event");
      if (!isEvent || !isRecord(part.data)) continue;
      const outcome = outcomeOf(part.data);
      if (!outcome) continue;
      const previous = outcomes.get(outcome.turnId);
      if (previous?.verified && !outcome.verified) continue;
      outcomes.set(outcome.turnId, outcome);
    }
  }
  return outcomes.size === 0 ? EMPTY : outcomes;
}
