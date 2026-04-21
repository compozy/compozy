import type { ReactElement } from "react";

import {
  Button,
  SectionHeading,
  StatusBadge,
  SurfaceCard,
  SurfaceCardBody,
  SurfaceCardDescription,
  SurfaceCardEyebrow,
  SurfaceCardHeader,
  SurfaceCardTitle,
  type StatusBadgeTone,
} from "@compozy/ui";
import { Link } from "@tanstack/react-router";

import { resolveStatusTone } from "./runs-list-view";

import type {
  RunJobState,
  RunShutdownState,
  RunSnapshot,
  RunTranscriptMessage,
  RunUsage,
} from "../types";
import type { RunStreamStatus } from "../hooks/use-run-stream";

export interface RunDetailViewProps {
  snapshot: RunSnapshot;
  isRefreshingSnapshot: boolean;
  streamStatus: RunStreamStatus;
  streamEventCount: number;
  lastHeartbeatAt: number | null;
  overflowReason?: string | null;
  streamError?: string | null;
  onReconnectStream: () => void;
  onCancelRun: () => void;
  cancelDisabled: boolean;
  isCancelling: boolean;
  cancelError?: string | null;
  cancelSuccess?: string | null;
}

export function RunDetailView(props: RunDetailViewProps): ReactElement {
  const {
    snapshot,
    isRefreshingSnapshot,
    streamStatus,
    streamEventCount,
    lastHeartbeatAt,
    overflowReason,
    streamError,
    onReconnectStream,
    onCancelRun,
    cancelDisabled,
    isCancelling,
    cancelError,
    cancelSuccess,
  } = props;

  const { run, jobs, transcript, shutdown, usage } = snapshot;
  const statusTone = resolveStatusTone(run.status);

  return (
    <div className="space-y-6" data-testid="run-detail-view">
      <SectionHeading
        actions={
          <div className="flex items-center gap-2">
            <StreamBadge status={streamStatus} />
            <Button
              data-testid="run-detail-reconnect"
              onClick={onReconnectStream}
              size="sm"
              variant="ghost"
            >
              Reconnect stream
            </Button>
            <Button
              data-testid="run-detail-cancel"
              disabled={cancelDisabled || isCancelling}
              onClick={onCancelRun}
              size="sm"
              variant="secondary"
            >
              {isCancelling ? "Cancelling…" : "Cancel run"}
            </Button>
          </div>
        }
        description={
          <span>
            {run.workflow_slug ? (
              <>
                <Link
                  className="underline-offset-4 hover:underline"
                  params={{ slug: run.workflow_slug }}
                  to="/workflows"
                >
                  {run.workflow_slug}
                </Link>
                {" · "}
              </>
            ) : null}
            {run.mode} · started {formatTimestamp(run.started_at)}
            {run.ended_at ? ` · ended ${formatTimestamp(run.ended_at)}` : " · in flight"}
          </span>
        }
        eyebrow={run.run_id}
        title={
          <span className="flex items-center gap-3">
            <span>{run.workflow_slug ?? run.run_id}</span>
            <StatusBadge data-testid="run-detail-status" tone={statusTone}>
              {run.status}
            </StatusBadge>
          </span>
        }
      />

      <StreamNotices
        eventCount={streamEventCount}
        heartbeatAt={lastHeartbeatAt}
        overflowReason={overflowReason ?? null}
        status={streamStatus}
        streamError={streamError ?? null}
      />

      {cancelError ? (
        <p
          className="rounded-[var(--radius-md)] border border-[color:var(--color-danger)] bg-black/20 px-4 py-3 text-sm text-[color:var(--color-danger)]"
          data-testid="run-detail-cancel-error"
          role="alert"
        >
          {cancelError}
        </p>
      ) : null}
      {cancelSuccess ? (
        <p
          className="rounded-[var(--radius-md)] border border-border bg-black/10 px-4 py-3 text-sm text-muted-foreground"
          data-testid="run-detail-cancel-success"
        >
          {cancelSuccess}
        </p>
      ) : null}

      {run.error_text ? (
        <p
          className="rounded-[var(--radius-md)] border border-[color:var(--color-danger)] bg-black/20 px-4 py-3 text-sm text-[color:var(--color-danger)]"
          data-testid="run-detail-error-text"
          role="alert"
        >
          {run.error_text}
        </p>
      ) : null}

      {shutdown ? <ShutdownCard shutdown={shutdown} /> : null}

      <div className="grid gap-4 xl:grid-cols-[minmax(0,1.1fr)_minmax(0,0.9fr)]">
        <JobsCard isRefreshing={isRefreshingSnapshot} jobs={jobs ?? []} />
        <UsageCard usage={usage} />
      </div>

      <TranscriptCard transcript={transcript ?? []} />
    </div>
  );
}

function StreamBadge({ status }: { status: RunStreamStatus }): ReactElement {
  const tone = statusToStreamTone(status);
  return (
    <StatusBadge data-testid="run-detail-stream-status" tone={tone}>
      stream {status}
    </StatusBadge>
  );
}

function statusToStreamTone(status: RunStreamStatus): StatusBadgeTone {
  switch (status) {
    case "open":
      return "success";
    case "connecting":
    case "reconnecting":
      return "accent";
    case "overflowed":
      return "warning";
    case "closed":
      return "neutral";
    default:
      return "info";
  }
}

function StreamNotices({
  eventCount,
  heartbeatAt,
  overflowReason,
  status,
  streamError,
}: {
  eventCount: number;
  heartbeatAt: number | null;
  overflowReason: string | null;
  status: RunStreamStatus;
  streamError: string | null;
}): ReactElement {
  return (
    <div
      className="grid gap-2 text-xs text-muted-foreground"
      data-testid="run-detail-stream-notices"
    >
      <p data-testid="run-detail-stream-events">events received · {eventCount}</p>
      {heartbeatAt ? (
        <p data-testid="run-detail-stream-heartbeat">
          last heartbeat · {new Date(heartbeatAt).toLocaleTimeString()}
        </p>
      ) : null}
      {status === "overflowed" ? (
        <p
          className="rounded-[var(--radius-md)] border border-[color:var(--color-warning)] bg-black/10 px-3 py-2 text-[color:var(--color-warning)]"
          data-testid="run-detail-stream-overflow"
          role="status"
        >
          Stream overflowed — snapshot was refreshed. {overflowReason ?? "Resume to continue."}
        </p>
      ) : null}
      {streamError ? (
        <p
          className="rounded-[var(--radius-md)] border border-[color:var(--color-danger)] bg-black/10 px-3 py-2 text-[color:var(--color-danger)]"
          data-testid="run-detail-stream-error"
          role="alert"
        >
          {streamError}
        </p>
      ) : null}
    </div>
  );
}

function ShutdownCard({ shutdown }: { shutdown: RunShutdownState }): ReactElement {
  return (
    <SurfaceCard data-testid="run-detail-shutdown">
      <SurfaceCardHeader>
        <div>
          <SurfaceCardEyebrow>Shutdown</SurfaceCardEyebrow>
          <SurfaceCardTitle>
            {shutdown.phase ? `Phase ${shutdown.phase}` : "Shutdown requested"}
          </SurfaceCardTitle>
          <SurfaceCardDescription>
            Requested {formatTimestamp(shutdown.requested_at)}
            {shutdown.source ? ` · by ${shutdown.source}` : null}
            {shutdown.deadline_at ? ` · deadline ${formatTimestamp(shutdown.deadline_at)}` : null}
          </SurfaceCardDescription>
        </div>
        <StatusBadge tone="warning">shutting down</StatusBadge>
      </SurfaceCardHeader>
    </SurfaceCard>
  );
}

function JobsCard({
  jobs,
  isRefreshing,
}: {
  jobs: RunJobState[];
  isRefreshing: boolean;
}): ReactElement {
  return (
    <SurfaceCard data-testid="run-detail-jobs">
      <SurfaceCardHeader>
        <div>
          <SurfaceCardEyebrow>Jobs</SurfaceCardEyebrow>
          <SurfaceCardTitle>Run jobs</SurfaceCardTitle>
          <SurfaceCardDescription>
            Per-job runtime status from the active run snapshot.
          </SurfaceCardDescription>
        </div>
        <StatusBadge tone="info">{jobs.length}</StatusBadge>
      </SurfaceCardHeader>
      <SurfaceCardBody>
        {jobs.length === 0 ? (
          <p className="text-sm text-muted-foreground" data-testid="run-detail-jobs-empty">
            No jobs reported yet.
          </p>
        ) : (
          <ul className="space-y-3" data-testid="run-detail-jobs-list">
            {jobs.map(job => (
              <JobRow job={job} key={job.job_id} />
            ))}
          </ul>
        )}
        {isRefreshing ? (
          <p
            className="mt-3 text-xs text-muted-foreground"
            data-testid="run-detail-jobs-refreshing"
          >
            refreshing snapshot…
          </p>
        ) : null}
      </SurfaceCardBody>
    </SurfaceCard>
  );
}

function JobRow({ job }: { job: RunJobState }): ReactElement {
  const tone = resolveStatusTone(job.status);
  return (
    <li
      className="flex items-center justify-between gap-3 rounded-[var(--radius-md)] border border-border bg-black/10 px-3 py-2"
      data-testid={`run-detail-job-row-${job.job_id}`}
    >
      <div className="min-w-0 space-y-1">
        <p className="truncate text-sm font-medium text-foreground">
          {job.summary?.task_title ?? job.task_id ?? job.job_id}
        </p>
        <p className="truncate text-xs text-muted-foreground">
          updated {formatTimestamp(job.updated_at)}
          {job.agent_name ? ` · ${job.agent_name}` : null}
        </p>
      </div>
      <StatusBadge data-testid={`run-detail-job-status-${job.job_id}`} tone={tone}>
        {job.status}
      </StatusBadge>
    </li>
  );
}

function UsageCard({ usage }: { usage?: RunUsage }): ReactElement {
  const entries: { label: string; value: number }[] = [
    { label: "Input tokens", value: usage?.input_tokens ?? 0 },
    { label: "Output tokens", value: usage?.output_tokens ?? 0 },
    { label: "Cache writes", value: usage?.cache_writes ?? 0 },
    { label: "Cache reads", value: usage?.cache_reads ?? 0 },
  ];
  return (
    <SurfaceCard data-testid="run-detail-usage">
      <SurfaceCardHeader>
        <div>
          <SurfaceCardEyebrow>Usage</SurfaceCardEyebrow>
          <SurfaceCardTitle>Token usage</SurfaceCardTitle>
          <SurfaceCardDescription>
            Aggregate token counters reported for this run.
          </SurfaceCardDescription>
        </div>
        <StatusBadge tone="info">{usage?.total_tokens ?? 0}</StatusBadge>
      </SurfaceCardHeader>
      <SurfaceCardBody className="grid grid-cols-2 gap-3">
        {entries.map(entry => (
          <div
            className="rounded-[var(--radius-md)] border border-border bg-black/10 px-3 py-2"
            data-testid={`run-detail-usage-${entry.label.toLowerCase().replace(/\s+/g, "-")}`}
            key={entry.label}
          >
            <p className="eyebrow text-muted-foreground">{entry.label}</p>
            <p className="mt-1 font-display text-lg tracking-[-0.02em] text-foreground">
              {entry.value}
            </p>
          </div>
        ))}
      </SurfaceCardBody>
    </SurfaceCard>
  );
}

function TranscriptCard({ transcript }: { transcript: RunTranscriptMessage[] }): ReactElement {
  return (
    <SurfaceCard data-testid="run-detail-transcript">
      <SurfaceCardHeader>
        <div>
          <SurfaceCardEyebrow>Transcript</SurfaceCardEyebrow>
          <SurfaceCardTitle>Recent messages</SurfaceCardTitle>
          <SurfaceCardDescription>
            Most recent persisted transcript entries from the daemon.
          </SurfaceCardDescription>
        </div>
        <StatusBadge tone="info">{transcript.length}</StatusBadge>
      </SurfaceCardHeader>
      <SurfaceCardBody>
        {transcript.length === 0 ? (
          <p className="text-sm text-muted-foreground" data-testid="run-detail-transcript-empty">
            Transcript is empty.
          </p>
        ) : (
          <ul className="space-y-3" data-testid="run-detail-transcript-list">
            {transcript.slice(-8).map(entry => (
              <li
                className="space-y-1 rounded-[var(--radius-md)] border border-border bg-black/10 px-3 py-2"
                data-testid={`run-detail-transcript-${entry.sequence}`}
                key={`${entry.sequence}-${entry.timestamp}`}
              >
                <p className="eyebrow text-muted-foreground">
                  {entry.role} · {formatTimestamp(entry.timestamp)}
                </p>
                <p className="text-sm text-foreground whitespace-pre-wrap">{entry.content}</p>
              </li>
            ))}
          </ul>
        )}
      </SurfaceCardBody>
    </SurfaceCard>
  );
}

function formatTimestamp(raw: string | undefined): string {
  if (!raw) {
    return "unknown";
  }
  try {
    return new Date(raw).toLocaleString();
  } catch {
    return raw;
  }
}
