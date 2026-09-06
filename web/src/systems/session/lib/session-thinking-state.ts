// Thinking state (ADR-006 rule 3): no assistant shell exists before content. From
// the send until the first token or tool the status row reads "Thinking…"; the
// reply mounts only once content exists. A very fast first token skips the
// thinking frame entirely so the row never flickers (US-023.EC-1); a failure
// before any content converts the pending reply into its error state.

import { isAgentEventPayload } from "./message-parts";
import {
  isSessionErrorEvent,
  isQueueRemovalMarker,
} from "../components/runtime-activity-notice.logic";
import { CLARIFY_EVENT_TYPE } from "./clarify-event";
import { PROMPT_STEERED_MARKER, PROMPT_SUPERSEDED_MARKER } from "./steer-marker";

/** A first token that lands within this window never shows a thinking frame. */
export const THINKING_FLICKER_GUARD_MS = 250;

export type SessionThinkingState = "none" | "pending" | "thinking" | "content" | "error";

export interface SessionThinkingInput {
  /** The turn is in flight (daemon or thread running). */
  running: boolean;
  /** The reply already has real content: a token, a tool call, reasoning, or a rich event. */
  hasContent: boolean;
  /** The turn ended in an error before any content arrived. */
  failed: boolean;
  /** When the operator sent the prompt; `null` when unknown (treat as past the guard). */
  sentAtMs: number | null;
  nowMs: number;
}

export function deriveThinkingState(input: SessionThinkingInput): SessionThinkingState {
  if (input.hasContent) return "content";
  if (input.failed) return "error";
  if (!input.running) return "none";
  if (input.sentAtMs !== null && input.nowMs - input.sentAtMs < THINKING_FLICKER_GUARD_MS) {
    return "pending";
  }
  return "thinking";
}

/** Milliseconds until a pending reply is allowed to show its thinking frame; 0 when due. */
export function thinkingGuardRemainingMs(sentAtMs: number | null, nowMs: number): number {
  if (sentAtMs === null) return 0;
  return Math.max(0, THINKING_FLICKER_GUARD_MS - (nowMs - sentAtMs));
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isCompozyEventPart(part: Record<string, unknown>): boolean {
  const type = part.type;
  if (type === "data-compozy-event") return true;
  return type === "data" && part.name === "compozy-event";
}

function isRichEventPart(part: Record<string, unknown>): boolean {
  if (part.type === "data-compozy-permission") return true;
  if (part.type === "data" && part.name === "compozy-permission") return true;
  if (!isCompozyEventPart(part)) return false;
  const data = part.data;
  if (!isAgentEventPayload(data)) return false;
  if (data.type === CLARIFY_EVENT_TYPE || isSessionErrorEvent(data)) return true;
  // A steer's provenance line or receipt bubble (VC-07) is content the reader
  // sees, even when the turn recorded nothing else.
  const kind = data.marker?.kind;
  return kind === PROMPT_STEERED_MARKER || kind === PROMPT_SUPERSEDED_MARKER;
}

/**
 * Whether an assistant message's parts amount to content the reader can see:
 * a non-empty text or reasoning token, any tool call, or a rich event row
 * (decision ask, permission, error). Operational markers alone do not mount a
 * reply — they are status, not the answer.
 */
export function assistantMessageHasContent(content: unknown): boolean {
  if (typeof content === "string") return content.trim().length > 0;
  if (!Array.isArray(content)) return false;
  return content.some(part => {
    if (typeof part === "string") return part.trim().length > 0;
    if (!isRecord(part)) return false;
    const type = part.type;
    if (type === "text" || type === "reasoning") {
      return typeof part.text === "string" && part.text.trim().length > 0;
    }
    if (type === "tool-call" || (typeof type === "string" && type.startsWith("tool-"))) {
      return true;
    }
    return isRichEventPart(part);
  });
}

/** Operational traces can mount a row without counting as the agent's reply. */
export function assistantMessageHasRenderableContent(content: unknown): boolean {
  if (assistantMessageHasContent(content)) return true;
  return (
    Array.isArray(content) &&
    content.some(part => {
      if (!isRecord(part) || !isCompozyEventPart(part) || !isAgentEventPayload(part.data))
        return false;
      return (
        part.data.marker?.kind === "transcript_marker.queue_cleared" ||
        isQueueRemovalMarker(part.data.marker)
      );
    })
  );
}
