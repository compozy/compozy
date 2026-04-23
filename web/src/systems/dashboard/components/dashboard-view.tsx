import type { ReactElement } from "react";

import {
  Activity,
  AlertTriangle,
  GitPullRequest,
  ListOrdered,
  RefreshCw,
  Server,
} from "lucide-react";

import {
  Alert,
  Button,
  EmptyState,
  Metric,
  SectionHeading,
  StatusBadge,
  SurfaceCard,
  SurfaceCardBody,
  SurfaceCardDescription,
  SurfaceCardEyebrow,
  SurfaceCardFooter,
  SurfaceCardHeader,
  SurfaceCardTitle,
  type StatusBadgeTone,
} from "@compozy/ui";
import { Link } from "@tanstack/react-router";

import { resolveStatusTone } from "@/systems/runs";
import type { Run } from "@/systems/runs";

import type { DashboardPayload, DashboardQueueSummary, WorkflowCard } from "../types";

type HealthDetail = NonNullable<DashboardPayload["health"]["details"]>[number];

export interface DashboardViewProps {
  dashboard: DashboardPayload;
  isRefetching: boolean;
  onSyncAll: () => void;
  isSyncing: boolean;
  lastSyncMessage?: string | null;
  lastSyncError?: string | null;
}

export function DashboardView({
  dashboard,
  isRefetching,
  onSyncAll,
  isSyncing,
  lastSyncMessage,
  lastSyncError,
}: DashboardViewProps): ReactElement {
  const { workspace, daemon, health, queue, pending_reviews, workflows, active_runs } = dashboard;
  const safeWorkflows = workflows ?? [];
  const safeRuns = (active_runs ?? []) as Run[];
  const healthTone = resolveHealthTone(health.ready, Boolean(health.degraded));
  const healthLabel = health.ready ? (health.degraded ? "degraded" : "ready") : "down";
  const daemonVersion = daemon.version ?? "unversioned";
  const daemonLocator = daemon.http_port ? `localhost:${daemon.http_port}` : `pid ${daemon.pid}`;
  return (
    <div className="space-y-6" data-testid="dashboard-view">
      <SectionHeading
        actions={
          <Button
            data-testid="dashboard-sync-all"
            disabled={isSyncing}
            icon={<RefreshCw className="size-4" />}
            loading={isSyncing}
            onClick={onSyncAll}
            size="sm"
          >
            Sync all workflows
          </Button>
        }
        description={`Daemon ${daemonVersion} · ${daemonLocator} · ${workspace.name}`}
        eyebrow="Overview"
        title="Operator dashboard"
      />

      {lastSyncError ? (
        <Alert data-testid="dashboard-sync-error" variant="error">
          {lastSyncError}
        </Alert>
      ) : null}
      {lastSyncMessage ? (
        <Alert data-testid="dashboard-sync-success" variant="success">
          {lastSyncMessage}
        </Alert>
      ) : null}

      <HealthDiagnostics degraded={Boolean(health.degraded)} details={health.details ?? []} />

      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <Metric
          data-testid="dashboard-stat-reviews"
          hint="awaiting review"
          label="Reviews"
          trailing={<GitPullRequest className="size-4 text-muted-foreground" aria-hidden />}
          value={pending_reviews}
        />
        <Metric
          data-testid="dashboard-stat-workflows"
          hint="tracked in workspace"
          label="Workflows"
          trailing={<ListOrdered className="size-4 text-muted-foreground" aria-hidden />}
          value={safeWorkflows.length}
        />
        <Metric
          data-testid="dashboard-stat-active-runs"
          hint={safeRuns.length === 1 ? "run in flight" : "runs in flight"}
          label="Active runs"
          trailing={<Activity className="size-4 text-muted-foreground" aria-hidden />}
          value={safeRuns.length}
        />
        <Metric
          data-testid="dashboard-stat-daemon"
          hint={daemonLocator}
          label="Daemon"
          trailing={
            <StatusBadge pulse={health.ready && !health.degraded} tone={healthTone}>
              {healthLabel}
            </StatusBadge>
          }
          value={healthLabel}
        />
      </div>

      <div className="grid gap-4 xl:grid-cols-[minmax(0,1.1fr)_minmax(0,0.9fr)]">
        <SurfaceCard data-testid="dashboard-workflows">
          <SurfaceCardHeader>
            <div>
              <SurfaceCardEyebrow>Workflows</SurfaceCardEyebrow>
              <SurfaceCardTitle>Workflow inventory</SurfaceCardTitle>
              <SurfaceCardDescription>
                Active workflows in this workspace. Drill in for tasks, runs, and reviews.
              </SurfaceCardDescription>
            </div>
            <StatusBadge tone="info">{safeWorkflows.length}</StatusBadge>
          </SurfaceCardHeader>
          <SurfaceCardBody>
            {safeWorkflows.length === 0 ? (
              <EmptyState
                data-testid="dashboard-workflows-empty"
                description={
                  <>
                    Register one with <code>compozy sync</code> or{" "}
                    <code>compozy workspace register</code>.
                  </>
                }
                icon={<ListOrdered className="size-4" aria-hidden />}
                title="No workflows yet"
              />
            ) : (
              <ul className="space-y-3" data-testid="dashboard-workflows-list">
                {safeWorkflows.slice(0, 6).map(card => (
                  <DashboardWorkflowRow card={card} key={card.workflow.id} />
                ))}
              </ul>
            )}
          </SurfaceCardBody>
          <SurfaceCardFooter>
            <Link
              className="text-xs font-semibold uppercase tracking-[0.12em] text-primary transition-colors hover:text-foreground"
              data-testid="dashboard-view-all-workflows"
              to="/workflows"
            >
              View all workflows →
            </Link>
            {isRefetching ? (
              <span className="text-xs text-muted-foreground">refreshing…</span>
            ) : null}
          </SurfaceCardFooter>
        </SurfaceCard>

        <ActiveRunsCard queue={queue} runs={safeRuns} />
      </div>
    </div>
  );
}

function HealthDiagnostics({
  degraded,
  details,
}: {
  degraded: boolean;
  details: HealthDetail[];
}): ReactElement | null {
  if (!degraded && details.length === 0) {
    return null;
  }
  return (
    <Alert
      data-testid="dashboard-health-diagnostics"
      icon={degraded ? <AlertTriangle className="size-4" /> : <Server className="size-4" />}
      title={degraded ? "Daemon health is degraded" : "Daemon health details"}
      variant={degraded ? "warning" : "neutral"}
    >
      {details.length === 0 ? (
        <span>The daemon reported a degraded state without additional detail.</span>
      ) : (
        <ul className="space-y-1">
          {details.map(detail => (
            <li className="flex flex-wrap gap-2" key={`${detail.code}-${detail.message}`}>
              <span className="font-mono text-xs text-current/90">{detail.code}</span>
              <span>{detail.message}</span>
              {detail.severity ? (
                <span className="text-current/70">({detail.severity})</span>
              ) : null}
            </li>
          ))}
        </ul>
      )}
    </Alert>
  );
}

function DashboardWorkflowRow({ card }: { card: WorkflowCard }): ReactElement {
  return (
    <li
      className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 rounded-[var(--radius-md)] border border-border-subtle bg-[color:var(--surface-inset)] px-3 py-2 transition-colors hover:border-border-strong hover:bg-surface-hover"
      data-testid={`dashboard-workflow-row-${card.workflow.slug}`}
    >
      <div className="min-w-0 space-y-1">
        <p className="truncate text-sm font-medium text-foreground">{card.workflow.slug}</p>
        <p className="truncate text-xs text-muted-foreground">
          {card.task_completed}/{card.task_total} tasks · {card.active_runs} active run
          {card.active_runs === 1 ? "" : "s"}
        </p>
      </div>
      <div className="flex items-center gap-2">
        <StatusBadge pulse={card.active_runs > 0} tone={card.active_runs > 0 ? "accent" : "info"}>
          {card.active_runs > 0 ? "running" : "idle"}
        </StatusBadge>
      </div>
    </li>
  );
}

function ActiveRunsCard({
  queue,
  runs,
}: {
  queue: DashboardQueueSummary;
  runs: Run[];
}): ReactElement {
  const visible = runs.slice(0, 5);
  return (
    <SurfaceCard data-testid="dashboard-active-runs">
      <SurfaceCardHeader>
        <div>
          <SurfaceCardEyebrow>Active runs</SurfaceCardEyebrow>
          <SurfaceCardTitle>Runs in flight</SurfaceCardTitle>
          <SurfaceCardDescription>
            Live view of currently-executing runs. Updates every few seconds.
          </SurfaceCardDescription>
        </div>
        <StatusBadge tone={runs.length > 0 ? "accent" : "info"}>{runs.length}</StatusBadge>
      </SurfaceCardHeader>
      <SurfaceCardBody>
        {visible.length === 0 ? (
          <EmptyState
            data-testid="dashboard-active-runs-empty"
            description="No active runs are streaming from the daemon."
            icon={<Activity className="size-4" aria-hidden />}
            title="Daemon is idle"
          />
        ) : (
          <ul className="space-y-2" data-testid="dashboard-active-runs-list">
            {visible.map(run => (
              <ActiveRunRow key={run.run_id} run={run} />
            ))}
          </ul>
        )}
      </SurfaceCardBody>
      <SurfaceCardFooter>
        <QueueSummaryChips queue={queue} />
        <Link
          className="text-xs font-semibold uppercase tracking-[0.12em] text-primary transition-colors hover:text-foreground"
          data-testid="dashboard-view-all-runs"
          to="/runs"
        >
          All runs →
        </Link>
      </SurfaceCardFooter>
    </SurfaceCard>
  );
}

function ActiveRunRow({ run }: { run: Run }): ReactElement {
  const tone = resolveStatusTone(run.status);
  return (
    <li
      className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 rounded-[var(--radius-md)] border border-border-subtle bg-[color:var(--surface-inset)] px-3 py-2 transition-colors hover:border-border-strong hover:bg-surface-hover"
      data-testid={`dashboard-active-run-${run.run_id}`}
    >
      <div className="min-w-0 space-y-1">
        <Link
          className="block truncate text-sm font-medium text-foreground hover:underline"
          data-testid={`dashboard-active-run-link-${run.run_id}`}
          params={{ runId: run.run_id }}
          to="/runs/$runId"
        >
          {run.workflow_slug ?? run.run_id}
        </Link>
        <p className="truncate text-xs text-muted-foreground">
          {run.mode} · started {formatTimestamp(run.started_at)}
        </p>
      </div>
      <StatusBadge pulse={tone === "accent"} tone={tone}>
        {run.status}
      </StatusBadge>
    </li>
  );
}

function QueueSummaryChips({ queue }: { queue: DashboardQueueSummary }): ReactElement {
  const entries: { label: string; value: number; tone: StatusBadgeTone }[] = [
    { label: "active", value: queue.active, tone: "accent" },
    { label: "completed", value: queue.completed, tone: "success" },
    { label: "failed", value: queue.failed, tone: "danger" },
    { label: "canceled", value: queue.canceled, tone: "warning" },
  ];
  return (
    <div className="flex flex-wrap items-center gap-1.5" data-testid="dashboard-queue-summary">
      {entries.map(entry => (
        <StatusBadge
          data-testid={`dashboard-queue-${entry.label}`}
          key={entry.label}
          tone={entry.tone}
        >
          {entry.value} {entry.label}
        </StatusBadge>
      ))}
    </div>
  );
}

function resolveHealthTone(ready: boolean, degraded: boolean): StatusBadgeTone {
  if (!ready) {
    return "danger";
  }
  if (degraded) {
    return "warning";
  }
  return "success";
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
