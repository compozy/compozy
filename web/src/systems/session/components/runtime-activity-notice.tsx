import { Activity, AlertCircle, AlertTriangle, Info } from "lucide-react";

import {
  Alert,
  AlertDescription,
  AlertMeta,
  AlertTitle,
  formatDuration as formatCanonicalDuration,
  Pill,
} from "@agh/ui";

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

const NOTICE_CLASS = "my-1.5 w-full min-w-0";

export function RuntimeActivityNotice({ event }: { event: AgentEventPayload }) {
  if (isSessionErrorEvent(event)) {
    const failureKind = event.failure?.kind?.trim();

    return (
      <Alert
        role="alert"
        data-testid="session-error-notice"
        data-tone="danger"
        className={NOTICE_CLASS}
        variant="danger"
      >
        <AlertCircle aria-hidden="true" className="mt-0.5 size-3.5 shrink-0" />
        <AlertTitle>Session failed</AlertTitle>
        {failureKind ? (
          <AlertMeta data-testid="session-error-meta">
            <Pill mono size="xs" tone="danger">
              {failureKind}
            </Pill>
          </AlertMeta>
        ) : null}
        <AlertDescription className="break-words" data-testid="session-error-detail">
          {sessionErrorDescription(event)}
        </AlertDescription>
      </Alert>
    );
  }

  if (isTranscriptMarkerEvent(event)) {
    const marker = markerFromEvent(event);
    const tone = markerTone(marker);
    const Icon = tone === "info" ? Info : AlertTriangle;
    return (
      <Alert
        role={tone === "info" ? "status" : "alert"}
        data-testid="transcript-marker-notice"
        data-tone={tone}
        className={NOTICE_CLASS}
        variant={tone === "danger" ? "danger" : tone === "warning" ? "warning" : "info"}
      >
        <Icon aria-hidden="true" className="mt-0.5 size-3.5 shrink-0" />
        <AlertTitle>Transcript marker</AlertTitle>
        <AlertMeta data-testid="transcript-marker-kind">
          <Pill
            mono
            size="xs"
            tone={tone === "danger" ? "danger" : tone === "warning" ? "warning" : "info"}
          >
            {markerLabel(marker, event)}
          </Pill>
        </AlertMeta>
        <AlertDescription className="break-words" data-testid="transcript-marker-summary">
          {marker?.summary || event.text || "Runtime marker recorded."}
        </AlertDescription>
      </Alert>
    );
  }

  if (!isRuntimeActivityEvent(event)) {
    return null;
  }

  const isWarning = event.type === "runtime_warning";
  const activity = event.runtime;
  const detail = describeActivity(activity);
  const meta = activityMeta(activity);

  // Progress stays a quiet in-thread meta row — WorkingIndicator owns activity chrome.
  if (!isWarning) {
    const title = event.text?.trim() || detail;
    return (
      <div
        role="status"
        data-testid="runtime-activity-notice"
        data-tone="progress"
        className="my-1 flex min-w-0 flex-wrap items-center gap-x-2 gap-y-0.5 px-1"
      >
        <Activity aria-hidden="true" className="size-3.5 shrink-0 text-subtle" />
        <span className="min-w-0 truncate text-small-body text-fg">{title}</span>
        {meta ? (
          <span
            className="text-form-hint text-muted tabular-nums"
            data-testid="runtime-activity-meta"
          >
            {meta}
          </span>
        ) : null}
        {title !== detail ? (
          <span
            className="min-w-0 truncate text-form-hint text-muted"
            data-testid="runtime-activity-detail"
          >
            {detail}
          </span>
        ) : (
          <span className="sr-only" data-testid="runtime-activity-detail">
            {detail}
          </span>
        )}
      </div>
    );
  }

  const title = event.text?.trim() || "Runtime warning";

  return (
    <Alert
      role="alert"
      data-testid="runtime-activity-notice"
      data-tone="warning"
      className={NOTICE_CLASS}
      variant="warning"
    >
      <AlertTriangle aria-hidden="true" className="mt-0.5 size-3.5 shrink-0" />
      <AlertTitle>{title}</AlertTitle>
      {meta ? (
        <AlertMeta data-testid="runtime-activity-meta">
          <span className="tabular-nums">{meta}</span>
        </AlertMeta>
      ) : null}
      <AlertDescription className="truncate" data-testid="runtime-activity-detail">
        {detail}
      </AlertDescription>
    </Alert>
  );
}
