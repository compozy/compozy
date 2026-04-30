import { Suspense, lazy, type ReactElement } from "react";

import { RefreshCw, XCircle } from "lucide-react";

import {
  Alert,
  Button,
  EmptyState,
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
import { RunEventFeed } from "./run-event-feed";
import type { RunFeedEvent } from "../lib/event-store";

import type { RunJobState, RunShutdownState, RunSnapshot, RunTranscript, RunUsage } from "../types";
import type { RunStreamStatus } from "../hooks/use-run-stream";

const RunTranscriptPanel = lazy(() =>
  import("./run-transcript-panel").then(module => ({ default: module.RunTranscriptPanel }))
);

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
  liveEvents?: readonly RunFeedEvent[];
  transcript?: RunTranscript;
  isLoadingTranscript?: boolean;
  isTranscriptError?: boolean;
  transcriptError?: string | null;
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
    liveEvents = [],
    transcript,
    isLoadingTranscript = false,
    isTranscriptError = false,
    transcriptError = null,
  } = props;

  const { run, jobs, shutdown, usage } = snapshot;
  const statusTone = resolveStatusTone(run.status);

  return (
    <div className="space-y-6" data-testid="run-detail-view">
      <SectionHeading
        actions={
          <div className="flex flex-wrap items-center gap-2">
            <StreamBadge status={streamStatus} />
            <Button
              data-testid="run-detail-reconnect"
              icon={<RefreshCw className="size-4" />}
              onClick={onReconnectStream}
              size="sm"
              variant="ghost"
            >
              Reconnect stream
            </Button>
            <Button
              data-testid="run-detail-cancel"
              disabled={cancelDisabled || isCancelling}
              icon={<XCircle className="size-4" />}
              loading={isCancelling}
              onClick={onCancelRun}
              size="sm"
              variant="secondary"
            >
              Cancel run
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
        <Alert data-testid="run-detail-cancel-error" variant="error">
          {cancelError}
        </Alert>
      ) : null}
      {cancelSuccess ? (
        <Alert data-testid="run-detail-cancel-success" variant="success">
          {cancelSuccess}
        </Alert>
      ) : null}

      {run.error_text ? (
        <Alert data-testid="run-detail-error-text" title="Run failed" variant="error">
          {run.error_text}
        </Alert>
      ) : null}

      {shutdown ? <ShutdownCard shutdown={shutdown} /> : null}

      <div className="grid gap-4 xl:grid-cols-[minmax(0,1.1fr)_minmax(0,0.9fr)]">
        <JobsCard isRefreshing={isRefreshingSnapshot} jobs={jobs ?? []} />
        <UsageCard usage={usage} />
      </div>

      <Suspense fallback={<TranscriptPanelFallback />}>
        <RunTranscriptPanel
          errorMessage={transcriptError}
          isError={isTranscriptError}
          isLoading={isLoadingTranscript}
          liveEvents={liveEvents}
          transcript={transcript}
        />
      </Suspense>

      <RunEventFeed events={liveEvents} />
    </div>
  );
}

function StreamBadge({ status }: { status: RunStreamStatus }): ReactElement {
  const tone = statusToStreamTone(status);
  return (
    <StatusBadge
      data-testid="run-detail-stream-status"
      pulse={tone === "accent" || tone === "success"}
      tone={tone}
    >
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
      className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground"
      data-testid="run-detail-stream-notices"
    >
      <StatusBadge data-testid="run-detail-stream-events" tone="neutral">
        {eventCount} events
      </StatusBadge>
      {heartbeatAt ? (
        <StatusBadge data-testid="run-detail-stream-heartbeat" tone="neutral">
          heartbeat {new Date(heartbeatAt).toLocaleTimeString()}
        </StatusBadge>
      ) : null}
      {status === "overflowed" ? (
        <Alert data-testid="run-detail-stream-overflow" variant="warning">
          Stream overflowed — snapshot was refreshed. {overflowReason ?? "Resume to continue."}
        </Alert>
      ) : null}
      {streamError ? (
        <Alert data-testid="run-detail-stream-error" variant="error">
          {streamError}
        </Alert>
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
          <EmptyState
            data-testid="run-detail-jobs-empty"
            description="The daemon has not reported a job snapshot for this run yet."
            title="No jobs reported yet"
          />
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
      className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 rounded-[var(--radius-md)] border border-border-subtle bg-[color:var(--surface-inset)] px-3 py-2 transition-colors hover:border-border-strong hover:bg-surface-hover"
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
      <StatusBadge
        data-testid={`run-detail-job-status-${job.job_id}`}
        pulse={tone === "accent"}
        tone={tone}
      >
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
      <SurfaceCardBody className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        {entries.map(entry => (
          <div
            className="rounded-[var(--radius-md)] border border-border-subtle bg-[color:var(--surface-inset)] px-3 py-2"
            data-testid={`run-detail-usage-${entry.label.toLowerCase().replace(/\s+/g, "-")}`}
            key={entry.label}
          >
            <p className="eyebrow text-muted-foreground">{entry.label}</p>
            <p className="mt-1 font-mono text-lg text-foreground tabular-nums">{entry.value}</p>
          </div>
        ))}
      </SurfaceCardBody>
    </SurfaceCard>
  );
}

function TranscriptPanelFallback(): ReactElement {
  return (
    <SurfaceCard data-testid="run-detail-transcript-loading">
      <SurfaceCardHeader>
        <div>
          <SurfaceCardEyebrow>Transcript</SurfaceCardEyebrow>
          <SurfaceCardTitle>Assistant log</SurfaceCardTitle>
          <SurfaceCardDescription>Loading structured transcript.</SurfaceCardDescription>
        </div>
        <StatusBadge tone="neutral">loading</StatusBadge>
      </SurfaceCardHeader>
      <SurfaceCardBody className="space-y-2">
        <div className="h-16 rounded-[var(--radius-md)] bg-muted" />
        <div className="h-24 rounded-[var(--radius-md)] bg-muted" />
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
