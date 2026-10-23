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

interface AssistantMessageFacts {
  traceCount: number | null;
  hasRenderableContent: boolean;
  /** Whether `AssistantMessage` mounts a row (no shell without content). */
  renders: boolean;
  turnId: string | undefined;
}

export interface AssistantMessagePlacement {
  traceCount: number | null;
  renders: boolean;
  hasRenderableContent: boolean;
  /** A drawn assistant message of the same turn sits directly above. */
  joinsPrevious: boolean;
  /** A drawn assistant message of the same turn sits directly below. */
  joinsNext: boolean;
}

// One index per loaded thread version and provenance: every mounted message
// reads its neighbours' facts from it instead of re-deriving them, and gets a
// stable placement object back while the thread is unchanged.
interface ThreadPlacementIndex {
  provenance: SteerProvenanceIndex;
  positions: Map<string, number>;
  facts: Map<number, AssistantMessageFacts>;
  placements: Map<string, AssistantMessagePlacement>;
}

const placementIndexes = new WeakMap<readonly AssistantThreadMessage[], ThreadPlacementIndex>();

function placementIndex(
  messages: readonly AssistantThreadMessage[],
  provenance: SteerProvenanceIndex
): ThreadPlacementIndex {
  const cached = placementIndexes.get(messages);
  if (cached?.provenance === provenance) return cached;
  const index: ThreadPlacementIndex = {
    provenance,
    positions: new Map(messages.map((message, position) => [message.id, position])),
    facts: new Map(),
    placements: new Map(),
  };
  placementIndexes.set(messages, index);
  return index;
}

function messageFacts(
  messages: readonly AssistantThreadMessage[],
  index: ThreadPlacementIndex,
  position: number
): AssistantMessageFacts {
  const cached = index.facts.get(position);
  if (cached) return cached;
  const message = messages[position]!;
  const traceCount = queueClearTraceCount(messages, message.id);
  const hasRenderableContent = assistantMessageHasRenderableContent(
    visibleAssistantContent(message.content, index.provenance)
  );
  const facts: AssistantMessageFacts = {
    traceCount,
    hasRenderableContent,
    renders: traceCount !== 0 && (hasRenderableContent || messageFailed(message)),
    turnId: toTimelineParts(message).find(part => part.turnId)?.turnId,
  };
  index.facts.set(position, facts);
  return facts;
}

// The nearest drawn neighbour in one direction continues this turn when it is an
// assistant message of the same turn. Anything else between them — an operator
// bubble, another session's card, a wake — ends the run.
function joinsNeighbour(
  messages: readonly AssistantThreadMessage[],
  index: ThreadPlacementIndex,
  position: number,
  step: 1 | -1,
  turnId: string
): boolean {
  for (let at = position + step; at >= 0 && at < messages.length; at += step) {
    if (messages[at]!.role !== "assistant") return false;
    const neighbour = messageFacts(messages, index, at);
    if (!neighbour.renders) continue;
    return neighbour.turnId === turnId;
  }
  return false;
}

const UNPLACED: AssistantMessagePlacement = {
  traceCount: null,
  renders: false,
  hasRenderableContent: false,
  joinsPrevious: false,
  joinsNext: false,
};

/**
 * Whether an assistant message draws, what it draws, and how it sits inside
 * its turn's run of assistant messages. Referentially stable for an unchanged
 * thread, so a selector can return it directly.
 */
export function assistantMessagePlacement(
  messages: readonly AssistantThreadMessage[],
  id: string,
  provenance: SteerProvenanceIndex
): AssistantMessagePlacement {
  const index = placementIndex(messages, provenance);
  const cached = index.placements.get(id);
  if (cached) return cached;
  const position = index.positions.get(id);
  if (position === undefined) return UNPLACED;
  const facts = messageFacts(messages, index, position);
  const turnId = facts.renders ? facts.turnId : undefined;
  const placement: AssistantMessagePlacement = {
    traceCount: facts.traceCount,
    renders: facts.renders,
    hasRenderableContent: facts.hasRenderableContent,
    joinsPrevious: turnId !== undefined && joinsNeighbour(messages, index, position, -1, turnId),
    joinsNext: turnId !== undefined && joinsNeighbour(messages, index, position, 1, turnId),
  };
  index.placements.set(id, placement);
  return placement;
}
