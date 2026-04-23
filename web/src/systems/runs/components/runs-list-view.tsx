import { useEffect, useMemo, useState, type ReactElement } from "react";

import {
  Alert,
  SectionHeading,
  SkeletonRow,
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

import type { Run, RunListModeFilter, RunListStatusFilter } from "../types";

export interface RunsListViewProps {
  runs: Run[];
  isLoading: boolean;
  isRefetching: boolean;
  error?: string | null;
  workspaceName: string;
  statusFilter: RunListStatusFilter;
  modeFilter: RunListModeFilter;
  onStatusChange: (next: RunListStatusFilter) => void;
  onModeChange: (next: RunListModeFilter) => void;
  degradedReason?: string | null;
}

const STATUS_OPTIONS: { value: RunListStatusFilter; label: string }[] = [
  { value: "all", label: "All" },
  { value: "active", label: "Active" },
  { value: "completed", label: "Completed" },
  { value: "failed", label: "Failed" },
  { value: "canceled", label: "Canceled" },
];

const MODE_OPTIONS: { value: RunListModeFilter; label: string }[] = [
  { value: "all", label: "Any mode" },
  { value: "task", label: "Task" },
  { value: "review", label: "Review" },
  { value: "exec", label: "Exec" },
];

const WORKFLOW_ALL = "all";

export function RunsListView(props: RunsListViewProps): ReactElement {
  const {
    runs,
    isLoading,
    isRefetching,
    error,
    workspaceName,
    statusFilter,
    modeFilter,
    onStatusChange,
    onModeChange,
    degradedReason,
  } = props;

  const [workflowFilter, setWorkflowFilter] = useState<string>(WORKFLOW_ALL);

  const workflowOptions = useMemo(() => {
    const slugs = new Set<string>();
    for (const run of runs) {
      if (run.workflow_slug) {
        slugs.add(run.workflow_slug);
      }
    }
    return [
      { value: WORKFLOW_ALL, label: "Any workflow" },
      ...Array.from(slugs)
        .sort()
        .map(slug => ({ value: slug, label: slug })),
    ];
  }, [runs]);

  const selectedWorkflowFilter = workflowOptions.some(option => option.value === workflowFilter)
    ? workflowFilter
    : WORKFLOW_ALL;

  useEffect(() => {
    if (workflowFilter !== selectedWorkflowFilter) {
      setWorkflowFilter(selectedWorkflowFilter);
    }
  }, [selectedWorkflowFilter, workflowFilter]);

  const visibleRuns = useMemo(() => {
    if (selectedWorkflowFilter === WORKFLOW_ALL) {
      return runs;
    }
    return runs.filter(run => run.workflow_slug === selectedWorkflowFilter);
  }, [runs, selectedWorkflowFilter]);

  return (
    <div className="space-y-6" data-testid="runs-list-view">
      <SectionHeading
        description={`Live and recent runs visible from ${workspaceName}.`}
        eyebrow="Runs"
        title="Run inventory"
      />

      <div
        className="flex flex-wrap items-center gap-3 rounded-[var(--radius-md)] border border-border bg-black/10 p-3"
        data-testid="runs-list-filters"
      >
        <FilterSelect<RunListStatusFilter>
          label="Status"
          options={STATUS_OPTIONS}
          value={statusFilter}
          onChange={onStatusChange}
          testId="runs-filter-status"
        />
        <FilterSelect<RunListModeFilter>
          label="Mode"
          options={MODE_OPTIONS}
          value={modeFilter}
          onChange={onModeChange}
          testId="runs-filter-mode"
        />
        <FilterSelect<string>
          label="Workflow"
          options={workflowOptions}
          value={selectedWorkflowFilter}
          onChange={setWorkflowFilter}
          testId="runs-filter-workflow"
        />
        {isRefetching ? (
          <span className="text-xs text-muted-foreground" data-testid="runs-list-refreshing">
            refreshing…
          </span>
        ) : null}
      </div>

      {degradedReason ? (
        <Alert data-testid="runs-list-degraded" variant="warning">
          {degradedReason}
        </Alert>
      ) : null}

      {error ? (
        <Alert data-testid="runs-list-error" variant="error">
          {error}
        </Alert>
      ) : null}

      {isLoading ? (
        <div aria-live="polite" className="space-y-2" data-testid="runs-list-loading" role="status">
          <p className="sr-only" data-testid="runs-list-loading-status">
            Loading runs…
          </p>
          <SkeletonRow />
          <SkeletonRow />
          <SkeletonRow />
        </div>
      ) : null}

      {!isLoading && visibleRuns.length === 0 && !error ? (
        <SurfaceCard data-testid="runs-list-empty">
          <SurfaceCardHeader>
            <div>
              <SurfaceCardEyebrow>Empty</SurfaceCardEyebrow>
              <SurfaceCardTitle>No matching runs</SurfaceCardTitle>
              <SurfaceCardDescription>
                No runs are currently visible with the selected filters. Start a workflow run from a
                workflow detail page to see it here.
              </SurfaceCardDescription>
            </div>
          </SurfaceCardHeader>
        </SurfaceCard>
      ) : null}

      {visibleRuns.length > 0 ? (
        <ul className="grid gap-3" data-testid="runs-list-items">
          {visibleRuns.map(run => (
            <RunRow key={run.run_id} run={run} />
          ))}
        </ul>
      ) : null}
    </div>
  );
}

function RunRow({ run }: { run: Run }): ReactElement {
  const tone = resolveStatusTone(run.status);
  const duration = computeDuration(run.started_at, run.ended_at);
  return (
    <li>
      <SurfaceCard data-testid={`runs-list-row-${run.run_id}`}>
        <SurfaceCardHeader>
          <div className="min-w-0">
            <SurfaceCardEyebrow>
              {run.mode} · {run.workflow_slug ?? "unknown workflow"}
            </SurfaceCardEyebrow>
            <SurfaceCardTitle>
              <Link
                className="text-foreground hover:underline"
                data-testid={`runs-list-link-${run.run_id}`}
                params={{ runId: run.run_id }}
                to="/runs/$runId"
              >
                {run.run_id}
              </Link>
            </SurfaceCardTitle>
            <SurfaceCardDescription>
              started {formatTimestamp(run.started_at)}
              {run.ended_at ? ` · ended ${formatTimestamp(run.ended_at)}` : " · in flight"}
              {duration ? ` · ${duration}` : ""}
            </SurfaceCardDescription>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            {duration ? (
              <span
                className="font-mono text-xs text-muted-foreground"
                data-testid={`runs-list-duration-${run.run_id}`}
              >
                {duration}
              </span>
            ) : null}
            <StatusBadge data-testid={`runs-list-status-${run.run_id}`} tone={tone}>
              {run.status}
            </StatusBadge>
          </div>
        </SurfaceCardHeader>
        {run.error_text ? (
          <SurfaceCardBody>
            <p
              className="text-sm text-[color:var(--tone-danger-text)]"
              data-testid={`runs-list-error-${run.run_id}`}
            >
              {run.error_text}
            </p>
          </SurfaceCardBody>
        ) : null}
      </SurfaceCard>
    </li>
  );
}

function FilterSelect<T extends string>({
  label,
  options,
  value,
  onChange,
  testId,
}: {
  label: string;
  options: { value: T; label: string }[];
  value: T;
  onChange: (next: T) => void;
  testId: string;
}): ReactElement {
  return (
    <label className="flex items-center gap-2 text-xs text-muted-foreground">
      <span className="font-eyebrow uppercase tracking-[0.14em]">{label}</span>
      <select
        className="rounded-[var(--radius-sm)] border border-border bg-card px-2 py-1 text-sm text-foreground shadow-[var(--shadow-xs)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/60"
        data-testid={testId}
        onChange={event => onChange(event.target.value as T)}
        value={value}
      >
        {options.map(option => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
    </label>
  );
}

export function resolveStatusTone(status: string): StatusBadgeTone {
  const normalized = status.trim().toLowerCase();
  switch (normalized) {
    case "running":
    case "queued":
    case "pending":
    case "retrying":
      return "accent";
    case "completed":
    case "succeeded":
    case "success":
      return "success";
    case "failed":
    case "crashed":
      return "danger";
    case "canceled":
    case "cancelled":
      return "warning";
    default:
      return "info";
  }
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

function computeDuration(
  startedAt: string | undefined,
  endedAt: string | undefined
): string | null {
  if (!startedAt) {
    return null;
  }
  const start = Date.parse(startedAt);
  if (Number.isNaN(start)) {
    return null;
  }
  const end = endedAt ? Date.parse(endedAt) : Date.now();
  if (Number.isNaN(end) || end < start) {
    return null;
  }
  const elapsed = Math.max(0, Math.round((end - start) / 1000));
  if (elapsed < 60) {
    return `${elapsed}s`;
  }
  if (elapsed < 3600) {
    const minutes = Math.floor(elapsed / 60);
    const seconds = elapsed % 60;
    return seconds === 0 ? `${minutes}m` : `${minutes}m ${seconds}s`;
  }
  const hours = Math.floor(elapsed / 3600);
  const minutes = Math.floor((elapsed % 3600) / 60);
  return minutes === 0 ? `${hours}h` : `${hours}h ${minutes}m`;
}
