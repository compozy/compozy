import { isProviderErrorEvent } from "../lib/provider-error";
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
  "shutdown",
]);

export function isSessionErrorEvent(event: AgentEventPayload): boolean {
  if (event.type !== "error") return false;
  if (isProviderErrorEvent(event) || hasText(event.failure?.summary)) return true;
  if (!hasText(event.error)) return false;
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
