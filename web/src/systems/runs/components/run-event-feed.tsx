import type { ReactElement } from "react";

import {
  EmptyState,
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
          <SurfaceCardEyebrow>Diagnostics</SurfaceCardEyebrow>
          <SurfaceCardTitle>Raw event feed</SurfaceCardTitle>
          <SurfaceCardDescription>
            Unformatted daemon events for debugging stream delivery and payload shape.
          </SurfaceCardDescription>
        </div>
        <StatusBadge tone="info">{total}</StatusBadge>
      </SurfaceCardHeader>
      <SurfaceCardBody>
        {visible.length === 0 ? (
          <EmptyState
            data-testid="run-event-feed-empty"
            description="New events will stream in as the daemon emits run, job, tool, and provider updates."
            title="No events received yet"
          />
        ) : (
          <ul
            className="relative space-y-0 before:absolute before:bottom-0 before:left-[0.8rem] before:top-0 before:w-px before:bg-border-subtle"
            data-testid="run-event-feed-list"
          >
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
      className="relative grid grid-cols-[1.6rem_minmax(0,1fr)] gap-3 py-2"
      data-kind={event.kind}
      data-testid={`run-event-feed-row-${event.id}`}
    >
      <span
        aria-hidden
        className="relative z-[1] mt-1 size-3 rounded-full border border-card bg-current"
        style={{ color: `var(--tone-${tone}-text)` }}
      />
      <div className="min-w-0 space-y-2 rounded-[var(--radius-md)] border border-border-subtle bg-[color:var(--surface-inset)] px-3 py-2 transition-colors hover:border-border-strong hover:bg-surface-hover">
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
        <details className="group">
          <summary className="cursor-pointer text-xs text-muted-foreground transition-colors hover:text-foreground">
            payload
          </summary>
          <pre className="mt-2 max-h-56 overflow-auto rounded-[var(--radius-sm)] border border-border-subtle bg-background/40 p-3 text-xs text-foreground">
            {formatPayload(event.payload)}
          </pre>
        </details>
      </div>
    </li>
  );
}

function formatPayload(payload: unknown): string {
  if (payload === undefined || payload === null) {
    return "{}";
  }
  try {
    return JSON.stringify(payload, null, 2);
  } catch {
    return String(payload);
  }
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
