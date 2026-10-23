// Whether an assistant message draws anything, and whether it continues the
// turn of its drawn neighbours. The daemon splits one turn into several
// messages around transcript markers (a steer delivered mid-turn lands between
// the turn's chunk and its `-2` continuation); presentation joins them back into
// one turn's rhythm. Each durable message keeps its identity and position.

import { assistantMessageHasRenderableContent } from "@/systems/session/lib/session-thinking-state";
import {
  PROMPT_ACCEPTED_MARKER,
  PROMPT_STEERED_MARKER,
  PROMPT_SUPERSEDED_MARKER,
} from "@/systems/session/lib/steer-marker";
import type { SteerProvenanceIndex } from "@/systems/session/lib/steer-provenance";
import { isRecord, toTimelineParts } from "@/systems/session/lib/timeline-message-parts";

import { queueClearTraceCount } from "./session-queue-trace.logic";

export interface AssistantThreadMessage {
  id: string;
  role: string;
  content?: unknown;
  status?: unknown;
}

const STEER_MARKER_KINDS = new Set([
  PROMPT_STEERED_MARKER,
  PROMPT_SUPERSEDED_MARKER,
  PROMPT_ACCEPTED_MARKER,
]);

function steerMarkerData(part: unknown): Record<string, unknown> | null {
  if (!isRecord(part)) return null;
  const isEvent =
    part.type === "data-compozy-event" || (part.type === "data" && part.name === "compozy-event");
  if (!isEvent || !isRecord(part.data) || !isRecord(part.data.marker)) return null;
  return STEER_MARKER_KINDS.has(String(part.data.marker.kind)) ? part.data : null;
}

/**
 * The message's content without the steer markers whose row draws nothing of
 * its own: an identity bound to its loaded message (the bubble's meta line owns
 * the fact) or an older marker of an identity whose latest marker renders
 * elsewhere. A marker row that resolves to a receipt or the legacy line stays.
 */
export function visibleAssistantContent(
  content: unknown,
  provenance: SteerProvenanceIndex
): unknown {
  if (!Array.isArray(content)) return content;
  const visible = content.filter(part => {
    const data = steerMarkerData(part);
    if (data === null) return true;
    const render = provenance.renderFor(data);
    return render === null || render.kind === "legacy" || render.kind === "receipt";
  });
  return visible.length === content.length ? content : visible;
}

function messageFailed(message: AssistantThreadMessage): boolean {
  const status = message.status;
  return isRecord(status) && status.type === "incomplete" && status.reason === "error";
}

/** Whether `AssistantMessage` mounts a row for this message (no shell without content). */
export function assistantMessageRenders(
  messages: readonly AssistantThreadMessage[],
  message: AssistantThreadMessage,
  provenance: SteerProvenanceIndex
): boolean {
  if (queueClearTraceCount(messages, message.id) === 0) return false;
  return (
    assistantMessageHasRenderableContent(visibleAssistantContent(message.content, provenance)) ||
    messageFailed(message)
  );
}

function assistantTurnId(message: AssistantThreadMessage): string | undefined {
  return toTimelineParts(message).find(part => part.turnId)?.turnId;
}

// The nearest drawn neighbour in one direction continues this turn when it is an
// assistant message of the same turn. Anything else between them — an operator
// bubble, another session's card, a wake — ends the run.
function joinsNeighbour(
  messages: readonly AssistantThreadMessage[],
  index: number,
  step: 1 | -1,
  turnId: string,
  provenance: SteerProvenanceIndex
): boolean {
  for (let at = index + step; at >= 0 && at < messages.length; at += step) {
    const neighbour = messages[at]!;
    if (neighbour.role !== "assistant") return false;
    if (!assistantMessageRenders(messages, neighbour, provenance)) continue;
    return assistantTurnId(neighbour) === turnId;
  }
  return false;
}

export interface AssistantTurnContinuity {
  /** A drawn assistant message of the same turn sits directly above. */
  joinsPrevious: boolean;
  /** A drawn assistant message of the same turn sits directly below. */
  joinsNext: boolean;
}

const UNJOINED: AssistantTurnContinuity = { joinsPrevious: false, joinsNext: false };

/** How a message sits inside its turn's run of assistant messages. */
export function assistantTurnContinuity(
  messages: readonly AssistantThreadMessage[],
  id: string,
  provenance: SteerProvenanceIndex
): AssistantTurnContinuity {
  const index = messages.findIndex(message => message.id === id);
  const message = messages[index];
  if (!message) return UNJOINED;
  const turnId = assistantTurnId(message);
  if (turnId === undefined) return UNJOINED;
  const joinsPrevious = joinsNeighbour(messages, index, -1, turnId, provenance);
  const joinsNext = joinsNeighbour(messages, index, 1, turnId, provenance);
  return joinsPrevious || joinsNext ? { joinsPrevious, joinsNext } : UNJOINED;
}
