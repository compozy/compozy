import type { ReactElement } from "react";

import {
  Button,
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

import type { DashboardPayload, DashboardQueueSummary, WorkflowCard } from "../types";

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
  const safeRuns = active_runs ?? [];
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
            onClick={onSyncAll}
            size="sm"
          >
            {isSyncing ? "Syncing…" : "Sync all workflows"}
          </Button>
        }
        description={`Daemon ${daemonVersion} · ${daemonLocator} · ${workspace.name}`}
        eyebrow="Overview"
        title="Operator dashboard"
      />

      {lastSyncError ? (
        <p
          className="rounded-[var(--radius-md)] border border-[color:var(--color-danger)] bg-black/20 px-4 py-3 text-sm text-[color:var(--color-danger)]"
          data-testid="dashboard-sync-error"
          role="alert"
        >
          {lastSyncError}
        </p>
      ) : null}
      {lastSyncMessage ? (
        <p
          className="rounded-[var(--radius-md)] border border-border bg-black/10 px-4 py-3 text-sm text-muted-foreground"
          data-testid="dashboard-sync-success"
        >
          {lastSyncMessage}
        </p>
      ) : null}

      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <StatCard
          description="Reviews awaiting operator attention."
          eyebrow="Reviews"
          title={pending_reviews.toString()}
          subtitle="pending"
        />
        <StatCard
          description="Workflows visible to the active workspace."
          eyebrow="Workflows"
          title={safeWorkflows.length.toString()}
          subtitle="tracked"
        />
        <StatCard
          description="Live runs currently in flight."
          eyebrow="Active runs"
          title={safeRuns.length.toString()}
          subtitle="running"
        />
        <StatCard
          description="Overall daemon readiness."
          eyebrow="Daemon"
          title={healthLabel}
          subtitle={daemonLocator}
          badge={<StatusBadge tone={healthTone}>{healthLabel}</StatusBadge>}
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
              <p className="text-sm text-muted-foreground" data-testid="dashboard-workflows-empty">
                No workflows yet. Register one through <code>compozy sync</code> or{" "}
                <code>compozy workspace register</code>.
              </p>
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
              className="text-xs font-semibold uppercase tracking-[0.12em] text-accent hover:underline"
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

        <QueueCard queue={queue} />
      </div>
    </div>
  );
}

function DashboardWorkflowRow({ card }: { card: WorkflowCard }): ReactElement {
  return (
    <li
      className="flex items-center justify-between gap-3 rounded-[var(--radius-md)] border border-border bg-black/10 px-3 py-2"
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
        <StatusBadge tone={card.active_runs > 0 ? "accent" : "info"}>
          {card.active_runs > 0 ? "running" : "idle"}
        </StatusBadge>
      </div>
    </li>
  );
}

function QueueCard({ queue }: { queue: DashboardQueueSummary }): ReactElement {
  const entries: { label: string; value: number; tone: StatusBadgeTone }[] = [
    { label: "Active", value: queue.active, tone: "accent" },
    { label: "Completed", value: queue.completed, tone: "success" },
    { label: "Failed", value: queue.failed, tone: "danger" },
    { label: "Canceled", value: queue.canceled, tone: "warning" },
  ];
  return (
    <SurfaceCard data-testid="dashboard-queue">
      <SurfaceCardHeader>
        <div>
          <SurfaceCardEyebrow>Queue</SurfaceCardEyebrow>
          <SurfaceCardTitle>Run queue</SurfaceCardTitle>
          <SurfaceCardDescription>
            Snapshot of queued and completed runs across this workspace.
          </SurfaceCardDescription>
        </div>
        <StatusBadge tone="info">total {queue.total}</StatusBadge>
      </SurfaceCardHeader>
      <SurfaceCardBody className="grid grid-cols-2 gap-3">
        {entries.map(entry => (
          <div
            className="rounded-[var(--radius-md)] border border-border bg-black/10 px-3 py-2"
            data-testid={`dashboard-queue-${entry.label.toLowerCase()}`}
            key={entry.label}
          >
            <p className="eyebrow text-muted-foreground">{entry.label}</p>
            <div className="mt-1 flex items-center gap-2">
              <span className="font-display text-2xl leading-none tracking-[-0.02em] text-foreground">
                {entry.value}
              </span>
              <StatusBadge tone={entry.tone}>{entry.label.toLowerCase()}</StatusBadge>
            </div>
          </div>
        ))}
      </SurfaceCardBody>
    </SurfaceCard>
  );
}

function StatCard({
  description,
  eyebrow,
  title,
  subtitle,
  badge,
}: {
  description: string;
  eyebrow: string;
  title: string;
  subtitle: string;
  badge?: ReactElement;
}): ReactElement {
  return (
    <SurfaceCard data-testid={`dashboard-stat-${eyebrow.toLowerCase().replace(/\s+/g, "-")}`}>
      <SurfaceCardHeader>
        <div>
          <SurfaceCardEyebrow>{eyebrow}</SurfaceCardEyebrow>
          <SurfaceCardTitle className="font-display tracking-[-0.02em]">{title}</SurfaceCardTitle>
          <SurfaceCardDescription>{description}</SurfaceCardDescription>
        </div>
        {badge ?? null}
      </SurfaceCardHeader>
      <SurfaceCardBody>
        <p className="eyebrow text-muted-foreground">{subtitle}</p>
      </SurfaceCardBody>
    </SurfaceCard>
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
