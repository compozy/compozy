import { isProviderErrorEvent } from "../lib/provider-error";
import { steerMarkerView } from "./steer-marker";
import type { AgentEventPayload, TranscriptMarkerPayload } from "../types";

const RUNTIME_EVENT_TYPES = new Set(["runtime_progress", "runtime_warning"]);
const OPERATIONAL_STATUS_EVENT_TYPES = new Set([
  "prompt_queued",
  "prompt_steered",
  "prompt_accepted",
  "prompt_dropped",
  "canceled",
]);
const TRANSCRIPT_MARKER_EVENT_TYPES = new Set([
  "transcript_marker.created",
  "transcript_marker.redacted",
]);

export function hasText(value: string | undefined): value is string {
  return typeof value === "string" && value.trim().length > 0;
}

export function isRuntimeActivityEvent(event: AgentEventPayload): boolean {
  return RUNTIME_EVENT_TYPES.has(event.type) && event.runtime !== undefined;
}

export function isOperationalStatusEvent(event: AgentEventPayload): boolean {
  return OPERATIONAL_STATUS_EVENT_TYPES.has(event.type);
}

// Stops the daemon attributes to supervision or to a request are not failures,
// even when the live tail carries them as an `error` event with the stop detail
// as its text (an inactivity stop reads "Stopped after … · no work for …", never
// "Session failed"). A failure stop reason, a failure record, or a provider
// diagnostic still is one.
const NON_FAILURE_STOP_REASONS = new Set([
  "timeout",
  "cancelled",
  "canceled",
  "interrupted",
  "aborted",
  "stopped",
  "user_canceled",
  "owner_released",
  "shutdown",
]);

/** The daemon's typed cancellation is a stop, including ACP request cancellation. */
export function isSessionCancellationEvent(event: AgentEventPayload): boolean {
  return event.type === "error" && event.failure?.kind === "cancellation" && !event.provider_error;
}

export function isSessionErrorEvent(event: AgentEventPayload): boolean {
  if (event.type !== "error" || isSessionCancellationEvent(event)) return false;
  if (isProviderErrorEvent(event) || hasText(event.failure?.summary)) return true;
  if (!hasText(event.error) && !hasText(event.text)) return false;
  const stopReason = event.stop_reason?.trim().toLowerCase();
  return !(stopReason && NON_FAILURE_STOP_REASONS.has(stopReason));
}

export function isTranscriptMarkerEvent(event: AgentEventPayload): boolean {
  return TRANSCRIPT_MARKER_EVENT_TYPES.has(event.type);
}

export function isFileMutationUnverifiedEvent(event: AgentEventPayload): boolean {
  if (!isTranscriptMarkerEvent(event)) return false;
  const rawKind =
    typeof event.raw === "object" && event.raw !== null && "kind" in event.raw
      ? event.raw.kind
      : undefined;
  return (
    (event.marker?.kind ?? rawKind ?? event.title) === "transcript_marker.file_mutation_unverified"
  );
}

export function isQueueRemovalMarker(marker: TranscriptMarkerPayload | null | undefined): boolean {
  return (
    marker?.kind === "transcript_marker.prompt_dropped" &&
    marker.evidence?.queue_status === "canceled" &&
    marker.evidence?.mode === "queue"
  );
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

export function markerFromEvent(event: AgentEventPayload): TranscriptMarkerPayload | null {
  if (event.marker) {
    return event.marker;
  }
  if (!isRecord(event.raw)) {
    return null;
  }
  const kind = typeof event.raw.kind === "string" ? event.raw.kind : event.title;
  const summary = typeof event.raw.summary === "string" ? event.raw.summary : event.text;
  const occurredAt =
    typeof event.raw.occurred_at === "string" ? event.raw.occurred_at : event.timestamp;
  if (!hasText(kind) || !hasText(summary) || !hasText(occurredAt)) {
    return null;
  }
  return {
    kind,
    summary,
    occurred_at: occurredAt,
    evidence: isRecord(event.raw.evidence) ? event.raw.evidence : undefined,
    diagnostic: event.raw.diagnostic,
  };
}

function isOperationalPromptKind(kind: string | undefined): boolean {
  const value = kind ?? "";
  return [
    "prompt_accepted",
    "prompt_queued",
    "prompt_steered",
    "prompt_interrupted",
    "prompt_dropped",
    "prompt_cancel",
    "prompt_canceled",
    "prompt_cancelled",
  ].some(operation => value.includes(operation));
}

/**
 * A transcript marker the transcript draws nothing for: an operational prompt
 * acknowledgement the input state already represents. A steer, a superseded
 * steer, a queued prompt reaching the turn, and a removed queued follow-up
 * keep their line (VC-07).
 */
export function isSilentTranscriptMarker(event: AgentEventPayload): boolean {
  if (!isTranscriptMarkerEvent(event)) return false;
  const marker = markerFromEvent(event);
  if (!marker || steerMarkerView(marker) || isQueueRemovalMarker(marker)) return false;
  return isOperationalPromptKind(marker.kind);
}
