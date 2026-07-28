import type { AgentEventPayload } from "../types";

const RUNTIME_EVENT_TYPES = new Set(["runtime_progress", "runtime_warning"]);
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

export function isSessionErrorEvent(event: AgentEventPayload): boolean {
  return event.type === "error" && (hasText(event.error) || hasText(event.failure?.summary));
}

export function isTranscriptMarkerEvent(event: AgentEventPayload): boolean {
  return TRANSCRIPT_MARKER_EVENT_TYPES.has(event.type);
}
