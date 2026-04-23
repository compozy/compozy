import type { ReactElement } from "react";

import {
  StatusBadge,
  SurfaceCard,
  SurfaceCardBody,
  SurfaceCardDescription,
  SurfaceCardEyebrow,
  SurfaceCardHeader,
  SurfaceCardTitle,
  type StatusBadgeTone,
} from "@compozy/ui";

import type { RunFeedEvent } from "../lib/event-store";

export interface RunEventFeedProps {
  events: readonly RunFeedEvent[];
  maxRows?: number;
}

const MAX_ROWS = 40;

export function RunEventFeed({ events, maxRows = MAX_ROWS }: RunEventFeedProps): ReactElement {
  const visible = events.slice(-maxRows).reverse();
  const total = events.length;
  return (
    <SurfaceCard data-testid="run-event-feed">
      <SurfaceCardHeader>
        <div>
          <SurfaceCardEyebrow>Live events</SurfaceCardEyebrow>
          <SurfaceCardTitle>Event feed</SurfaceCardTitle>
          <SurfaceCardDescription>
            Stream of the most recent events emitted by this run.
          </SurfaceCardDescription>
        </div>
        <StatusBadge tone="info">{total}</StatusBadge>
      </SurfaceCardHeader>
      <SurfaceCardBody>
        {visible.length === 0 ? (
          <p className="text-sm text-muted-foreground" data-testid="run-event-feed-empty">
            No events received yet. New events stream in as the daemon emits them.
          </p>
        ) : (
          <ul className="space-y-2" data-testid="run-event-feed-list">
            {visible.map(event => (
              <EventRow event={event} key={event.id} />
            ))}
          </ul>
        )}
      </SurfaceCardBody>
    </SurfaceCard>
  );
}

function EventRow({ event }: { event: RunFeedEvent }): ReactElement {
  const tone = toneForKind(event.kind);
  const timestamp = event.timestamp ?? new Date(event.receivedAt).toISOString();
  const summary = summarizePayload(event.kind, event.payload);
  return (
    <li
      className="flex items-start justify-between gap-3 rounded-[var(--radius-md)] border border-border bg-black/10 px-3 py-2"
      data-kind={event.kind}
      data-testid={`run-event-feed-row-${event.id}`}
    >
      <div className="min-w-0 flex-1 space-y-1">
        <div className="flex flex-wrap items-center gap-2">
          <StatusBadge tone={tone}>{event.kind}</StatusBadge>
          <span className="font-mono text-[11px] text-muted-foreground">
            {formatTimestamp(timestamp)}
          </span>
          {event.seq !== null ? (
            <span className="font-mono text-[11px] text-muted-foreground/80">seq {event.seq}</span>
          ) : null}
        </div>
        {summary ? (
          <p className="truncate text-xs text-foreground/90" title={summary}>
            {summary}
          </p>
        ) : null}
      </div>
    </li>
  );
}

function toneForKind(kind: string): StatusBadgeTone {
  if (kind.startsWith("run.")) {
    if (kind === "run.completed") return "success";
    if (kind === "run.failed" || kind === "run.crashed") return "danger";
    if (kind === "run.cancelled") return "warning";
    return "accent";
  }
  if (kind.startsWith("job.")) {
    if (kind.endsWith(".failed") || kind === "job.cancelled") return "danger";
    if (kind.endsWith(".completed")) return "success";
    return "accent";
  }
  if (kind.startsWith("session.")) {
    if (kind === "session.failed") return "danger";
    if (kind === "session.completed") return "success";
    return "info";
  }
  if (kind.startsWith("tool_call.")) {
    if (kind === "tool_call.failed") return "danger";
    return "info";
  }
  if (kind.startsWith("provider.")) {
    if (kind === "provider.call_failed") return "danger";
    return "info";
  }
  if (kind.startsWith("shutdown.")) {
    return "warning";
  }
  return "neutral";
}

function summarizePayload(kind: string, payload: unknown): string | null {
  if (!payload || typeof payload !== "object") {
    return null;
  }
  const record = payload as Record<string, unknown>;
  const prefer: Record<string, string[]> = {
    "tool_call.started": ["tool_name", "name", "tool"],
    "tool_call.updated": ["tool_name", "name", "tool", "status"],
    "tool_call.failed": ["tool_name", "name", "tool", "error"],
    "session.update": ["summary", "message", "status"],
    "job.started": ["task_id", "task_title", "summary"],
    "job.attempt_started": ["task_id", "task_title", "attempt"],
    "job.attempt_finished": ["task_id", "status", "error"],
    "job.completed": ["task_id", "task_title"],
    "job.failed": ["task_id", "error"],
    "usage.updated": ["total_tokens", "input_tokens", "output_tokens"],
    "usage.aggregated": ["total_tokens"],
  };
  const candidates = prefer[kind] ?? ["summary", "message", "title", "error"];
  const parts: string[] = [];
  for (const key of candidates) {
    const value = record[key];
    if (value === undefined || value === null) {
      continue;
    }
    const asString = typeof value === "string" ? value : JSON.stringify(value);
    parts.push(`${key}=${asString}`);
    if (parts.length >= 3) break;
  }
  if (parts.length === 0) {
    return null;
  }
  return parts.join(" · ");
}

function formatTimestamp(raw: string | null): string {
  if (!raw) return "";
  try {
    const d = new Date(raw);
    return d.toLocaleTimeString(undefined, {
      hour12: false,
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
    });
  } catch {
    return raw;
  }
}
