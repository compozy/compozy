import { Activity, AlertCircle, AlertTriangle, Info } from "lucide-react";

import { formatDuration as formatCanonicalDuration, Marker, MarkerMeta } from "@compozy/ui";

import type { AgentEventPayload, RuntimeActivityPayload, TranscriptMarkerPayload } from "../types";
import {
  hasText,
  isRuntimeActivityEvent,
  isSessionErrorEvent,
  isTranscriptMarkerEvent,
} from "./runtime-activity-notice.logic";

function formatDuration(seconds: number | undefined): string | null {
  if (typeof seconds !== "number" || !Number.isFinite(seconds) || seconds < 0) {
    return null;
  }
  return formatCanonicalDuration(Math.floor(seconds) * 1_000);
}

function humanizeKind(kind: string | undefined): string | null {
  const normalized = kind?.trim();
  if (!normalized) {
    return null;
  }
  return normalized.replaceAll("_", " ");
}

function describeActivity(activity: RuntimeActivityPayload | undefined): string {
  if (!activity) {
    return "Waiting for runtime activity";
  }

  if (activity.current_tool?.trim()) {
    return `Using ${activity.current_tool.trim()}`;
  }

  if (activity.last_activity_detail?.trim()) {
    return activity.last_activity_detail.trim();
  }

  return humanizeKind(activity.last_activity_kind) ?? "Runtime activity";
}

function activityMeta(activity: RuntimeActivityPayload | undefined): string | null {
  const elapsed = formatDuration(activity?.elapsed_seconds);
  const idle = formatDuration(activity?.idle_seconds);
  if (elapsed && idle) {
    return `${elapsed} elapsed · ${idle} idle`;
  }
  if (elapsed) {
    return `${elapsed} elapsed`;
  }
  if (idle) {
    return `${idle} idle`;
  }
  return null;
}

function normalizeErrorText(error: string | undefined): string | null {
  if (!hasText(error)) {
    return null;
  }

  const trimmed = error.trim();
  try {
    const parsed: unknown = JSON.parse(trimmed);
    if (typeof parsed === "object" && parsed !== null && "data" in parsed) {
      const data = (parsed as { data?: unknown }).data;
      if (typeof data === "object" && data !== null && "error" in data) {
        const nested = (data as { error?: unknown }).error;
        if (typeof nested === "string" && nested.trim().length > 0) {
          return nested.trim();
        }
      }
    }
    if (typeof parsed === "object" && parsed !== null && "message" in parsed) {
      const message = (parsed as { message?: unknown }).message;
      if (typeof message === "string" && message.trim().length > 0) {
        return message.trim();
      }
    }
  } catch {
    return trimmed;
  }

  return trimmed;
}

function sessionErrorDescription(event: AgentEventPayload): string {
  return (
    normalizeErrorText(event.error) ||
    normalizeErrorText(event.failure?.summary) ||
    "The session stopped before completing this turn."
  );
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function markerFromEvent(event: AgentEventPayload): TranscriptMarkerPayload | null {
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

function markerTone(marker: TranscriptMarkerPayload | null) {
  const kind = marker?.kind ?? "";
  if (kind.includes("failure") || kind.includes("timeout") || kind.includes("interrupted")) {
    return "danger" as const;
  }
  if (kind.includes("recovered")) {
    return "info" as const;
  }
  return "warning" as const;
}

function markerLabel(marker: TranscriptMarkerPayload | null, event: AgentEventPayload): string {
  return marker?.kind || event.title || event.type;
}

/** Faint ×N mono count appended when consecutive same-kind events clustered. */
function ClusterCount({ count }: { count: number }) {
  if (count <= 1) return null;
  return <MarkerMeta data-testid="marker-cluster-count"> ×{count}</MarkerMeta>;
}

/**
 * Runtime events as one-line markers — the calm replacement for the old tinted
 * Alert cards. Tone lives in the 12px glyph; the raw kind string renders as
 * faint mono meta, never as a pill; consecutive same-kind events arrive
 * pre-clustered with a ×N count.
 */
export function RuntimeActivityNotice({
  event,
  count = 1,
}: {
  event: AgentEventPayload;
  count?: number;
}) {
  if (isSessionErrorEvent(event)) {
    const failureKind = event.failure?.kind?.trim();

    return (
      <Marker
        role="alert"
        data-testid="session-error-notice"
        tone="danger"
        icon={<AlertCircle strokeWidth={1.8} />}
      >
        <b>Session failed</b> —{" "}
        <span data-testid="session-error-detail">{sessionErrorDescription(event)}</span>
        {failureKind ? (
          <>
            {" "}
            <MarkerMeta data-testid="session-error-meta">{failureKind}</MarkerMeta>
          </>
        ) : null}
        <ClusterCount count={count} />
      </Marker>
    );
  }

  if (isTranscriptMarkerEvent(event)) {
    const marker = markerFromEvent(event);
    const tone = markerTone(marker);
    const Icon = tone === "info" ? Info : AlertTriangle;
    return (
      <Marker
        role={tone === "info" ? "status" : "alert"}
        data-testid="transcript-marker-notice"
        data-marker-tone={tone}
        tone={tone}
        icon={<Icon strokeWidth={1.8} />}
      >
        <span data-testid="transcript-marker-summary">
          {marker?.summary || event.text || "Runtime marker recorded."}
        </span>{" "}
        <MarkerMeta data-testid="transcript-marker-kind">{markerLabel(marker, event)}</MarkerMeta>
        <ClusterCount count={count} />
      </Marker>
    );
  }

  if (!isRuntimeActivityEvent(event)) {
    return null;
  }

  const isWarning = event.type === "runtime_warning";
  const activity = event.runtime;
  const detail = describeActivity(activity);
  const meta = activityMeta(activity);

  if (!isWarning) {
    const title = event.text?.trim() || detail;
    return (
      <Marker
        role="status"
        tone="neutral"
        data-testid="runtime-activity-notice"
        icon={<Activity strokeWidth={1.8} />}
      >
        <b>{title}</b>
        {meta ? (
          <>
            {" "}
            <MarkerMeta data-testid="runtime-activity-meta">{meta}</MarkerMeta>
          </>
        ) : null}{" "}
        {title !== detail ? (
          <span data-testid="runtime-activity-detail">{detail}</span>
        ) : (
          <span className="sr-only" data-testid="runtime-activity-detail">
            {detail}
          </span>
        )}
        <ClusterCount count={count} />
      </Marker>
    );
  }

  const title = event.text?.trim() || "Runtime warning";

  return (
    <Marker
      role="alert"
      data-testid="runtime-activity-notice"
      data-marker-tone="warning"
      tone="warning"
      icon={<AlertTriangle strokeWidth={1.8} />}
    >
      <b>{title}</b> — <span data-testid="runtime-activity-detail">{detail}</span>
      {meta ? (
        <>
          {" "}
          <MarkerMeta data-testid="runtime-activity-meta">{meta}</MarkerMeta>
        </>
      ) : null}
      <ClusterCount count={count} />
    </Marker>
  );
}
