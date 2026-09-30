import { Activity, AlertCircle, AlertTriangle, Info, ListX, ScrollText } from "lucide-react";

import { formatDuration as formatCanonicalDuration, Marker, MarkerMeta } from "@compozy/ui";

import { getToolLabel, resolveRegisteredToolName } from "../lib/tool-labels";
import { steerMarkerView } from "../lib/steer-marker";
import { SteerMarkerRow } from "./steer-marker-notice";
import { providerErrorView } from "../lib/provider-error";
import { ClusterCount } from "./marker-cluster-count";
import { ProviderErrorNotice } from "./provider-error-notice";
import type { AgentEventPayload, RuntimeActivityPayload, TranscriptMarkerPayload } from "../types";
import {
  hasText,
  isOperationalStatusEvent,
  isQueueRemovalMarker,
  isRuntimeActivityEvent,
  isSessionErrorEvent,
  isTranscriptMarkerEvent,
} from "@/systems/session/lib/runtime-activity-notice";

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
    return "Waiting for the agent…";
  }

  const tool = activity.current_tool?.trim();
  if (tool) {
    return getToolLabel(resolveRegisteredToolName(tool), "active");
  }

  if (activity.last_activity_detail?.trim()) {
    return activity.last_activity_detail.trim();
  }

  return humanizeKind(activity.last_activity_kind) ?? "Agent activity";
}

// One plain duration: how long the agent has been at it. Idle time is a
// diagnostic the status row already owns.
function activityMeta(activity: RuntimeActivityPayload | undefined): string | null {
  const elapsed = formatDuration(activity?.elapsed_seconds);
  return elapsed ? `for ${elapsed}` : null;
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
    normalizeErrorText(event.text) ||
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
  if (kind.includes("failure") || kind.includes("interrupted")) {
    return "danger" as const;
  }
  if (kind.includes("timeout")) {
    // The daemon's timeout marker names its evidence: a failure kind earns
    // danger; a supervisor stop (no failure kind, `stop_reason: timeout`) is
    // a warning — the session stopped, nothing broke.
    const failureKind = marker?.evidence?.failure_kind;
    const supervised = marker?.evidence?.stop_reason === "timeout" && failureKind === "";
    return supervised ? ("warning" as const) : ("danger" as const);
  }
  if (kind.includes("recovered")) {
    return "info" as const;
  }
  return "warning" as const;
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

function markerLabel(marker: TranscriptMarkerPayload | null, event: AgentEventPayload): string {
  return marker?.kind || event.title || event.type;
}

const POST_STOP_MARKER = "transcript_marker.post_stop";
const QUEUE_CLEARED_MARKER = "transcript_marker.queue_cleared";

function evidenceString(marker: TranscriptMarkerPayload, key: string): string | null {
  const value = marker.evidence?.[key];
  return typeof value === "string" && value.trim().length > 0 ? value.trim() : null;
}

/** Who cleared the queue, from the marker's actor evidence: the operator reads as "You". */
function queueClearActor(marker: TranscriptMarkerPayload): string {
  const kind = evidenceString(marker, "actor_kind");
  const id = evidenceString(marker, "actor_id");
  if (kind === null || kind === "user" || kind === "human") return "You";
  return id ? `${kind} ${id}` : kind;
}

// One entry removed by an explicit clear (ADR-003): neutral — nothing failed,
// the operator (or another actor) asked for it. One marker per entry, so the
// cluster count says how many left together.
function QueueClearedMarkerNotice({
  marker,
  count,
}: {
  marker: TranscriptMarkerPayload;
  count: number;
}) {
  return (
    <Marker
      role="status"
      data-testid="transcript-marker-notice"
      data-marker-tone="neutral"
      tone="neutral"
      icon={<ListX strokeWidth={1.8} />}
    >
      <span data-testid="transcript-marker-summary">
        <b>{queueClearActor(marker)} cleared the queue</b> — a queued follow-up was removed
      </span>{" "}
      <ClusterCount count={count} />
    </Marker>
  );
}

// Late provider output after a verified stop: the daemon discards the event's
// content and keeps only this marker, one per turn. Neutral — nothing failed and
// the turn is not resurrected — the sentence names the consequence without
// promising recoverable output; the raw kind stays off screen in `data-marker-kind`.
function PostStopMarkerNotice({ label, count }: { label: string; count: number }) {
  return (
    <Marker
      role="status"
      data-testid="transcript-marker-notice"
      data-marker-kind={label}
      data-marker-tone="neutral"
      tone="neutral"
      icon={<ScrollText strokeWidth={1.8} />}
    >
      <span data-testid="transcript-marker-summary">
        <b>The agent sent more output after you stopped it</b> — discarded; the reply was not
        changed.
      </span>{" "}
      <ClusterCount count={count} />
    </Marker>
  );
}

/** A session-level failure: the provider-shaped notice when the daemon named one, else the generic marker. */
function SessionErrorNotice({ event, count }: { event: AgentEventPayload; count: number }) {
  const provider = providerErrorView(event);
  if (provider) {
    return <ProviderErrorNotice view={provider} count={count} />;
  }
  const failureKind = event.failure?.kind?.trim();

  return (
    <Marker
      role="alert"
      data-testid="session-error-notice"
      data-failure-kind={failureKind || undefined}
      tone="danger"
      icon={<AlertCircle strokeWidth={1.8} />}
    >
      <b>Session failed</b> —{" "}
      <span data-testid="session-error-detail">{sessionErrorDescription(event)}</span>
      <ClusterCount count={count} />
    </Marker>
  );
}

/** A transcript marker: operational prompt kinds stay silent, post-stop discards read neutral, the rest by tone. */
function TranscriptMarkerNotice({ event, count }: { event: AgentEventPayload; count: number }) {
  const marker = markerFromEvent(event);
  // A steer, a superseded steer, or a queued prompt reaching the turn is the
  // one lifecycle line the transcript keeps (VC-07); other acknowledgements
  // are already represented by the input state.
  const steer = marker ? steerMarkerView(marker) : null;
  if (steer) {
    return <SteerMarkerRow event={event} view={steer} count={count} />;
  }
  if (isQueueRemovalMarker(marker)) {
    return (
      <Marker
        role="status"
        data-testid="transcript-marker-notice"
        tone="neutral"
        icon={<ListX strokeWidth={1.8} />}
      >
        <span data-testid="transcript-marker-summary">
          <b>You removed a queued follow-up</b>
        </span>
        <ClusterCount count={count} />
      </Marker>
    );
  }
  if (isOperationalPromptKind(marker?.kind)) {
    return null;
  }
  if (marker?.kind === POST_STOP_MARKER) {
    return <PostStopMarkerNotice label={markerLabel(marker, event)} count={count} />;
  }
  if (marker?.kind === QUEUE_CLEARED_MARKER) {
    return <QueueClearedMarkerNotice marker={marker} count={count} />;
  }
  const tone = markerTone(marker);
  const Icon = tone === "info" ? Info : AlertTriangle;
  return (
    <Marker
      role={tone === "info" ? "status" : "alert"}
      data-testid="transcript-marker-notice"
      data-marker-kind={markerLabel(marker, event)}
      data-marker-tone={tone}
      tone={tone}
      icon={<Icon strokeWidth={1.8} />}
    >
      <span data-testid="transcript-marker-summary">
        {marker?.summary || event.text || "Agent update"}
      </span>{" "}
      <ClusterCount count={count} />
    </Marker>
  );
}

/** Plain runtime activity: the event text leads when present, the derived detail stays for readers. */
function RuntimeActivityMarker({
  event,
  count,
  detail,
  meta,
}: {
  event: AgentEventPayload;
  count: number;
  detail: string;
  meta: string | null;
}) {
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

/**
 * Runtime events as one-line markers — the calm replacement for the old tinted
 * Alert cards. Tone lives in the 12px glyph; raw kind strings never reach the
 * screen (they ride on `data-marker-kind` / `data-failure-kind` for
 * diagnostics); consecutive same-kind events arrive pre-clustered with a ×N
 * count.
 */
export function RuntimeActivityNotice({
  event,
  count = 1,
}: {
  event: AgentEventPayload;
  count?: number;
}) {
  if (isSessionErrorEvent(event)) {
    return <SessionErrorNotice event={event} count={count} />;
  }
  if (isTranscriptMarkerEvent(event)) {
    return <TranscriptMarkerNotice event={event} count={count} />;
  }
  if (isOperationalStatusEvent(event) || !isRuntimeActivityEvent(event)) {
    return null;
  }

  const activity = event.runtime;
  const detail = describeActivity(activity);
  const meta = activityMeta(activity);
  if (event.type !== "runtime_warning") {
    return <RuntimeActivityMarker event={event} count={count} detail={detail} meta={meta} />;
  }

  const title = event.text?.trim() || "Warning";

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
