import { useEffect, useMemo, useState, type ReactElement } from "react";

import { Activity, FilterX } from "lucide-react";

import {
  Alert,
  EmptyState,
  SectionHeading,
  SkeletonRow,
  StatusBadge,
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
        className="flex flex-wrap items-end gap-3 rounded-[var(--radius-xl)] border border-border-subtle bg-card p-3 shadow-[var(--shadow-sm)]"
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
          <StatusBadge data-testid="runs-list-refreshing" pulse tone="accent">
            refreshing…
          </StatusBadge>
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
        <EmptyState
          data-testid="runs-list-empty"
          description="No runs are visible with the selected filters. Start a workflow run from a workflow detail page or loosen the filters."
          icon={<FilterX className="size-4" aria-hidden />}
          title="No matching runs"
        />
      ) : null}

      {visibleRuns.length > 0 ? (
        <ul
          className="overflow-hidden rounded-[var(--radius-xl)] border border-border-subtle bg-card shadow-[var(--shadow-sm)]"
          data-testid="runs-list-items"
        >
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
    <li
      className="group border-b border-border-subtle last:border-b-0"
      data-testid={`runs-list-row-${run.run_id}`}
    >
      <div className="grid gap-3 px-4 py-3 transition-colors group-hover:bg-surface-hover md:grid-cols-[minmax(0,1.2fr)_minmax(160px,0.55fr)_minmax(120px,0.35fr)_auto] md:items-center">
        <div className="min-w-0 space-y-1">
          <p className="eyebrow text-muted-foreground">
            {run.mode} · {run.workflow_slug ?? "unknown workflow"}
          </p>
          <Link
            className="block truncate font-mono text-sm text-foreground transition-colors hover:text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/60"
            data-testid={`runs-list-link-${run.run_id}`}
            params={{ runId: run.run_id }}
            title={run.run_id}
            to="/runs/$runId"
          >
            {run.run_id}
          </Link>
          {run.error_text ? (
            <p
              className="line-clamp-2 text-xs text-[color:var(--tone-danger-text)]"
              data-testid={`runs-list-error-${run.run_id}`}
              title={run.error_text}
            >
              {run.error_text}
            </p>
          ) : null}
        </div>
        <div className="min-w-0 text-xs text-muted-foreground">
          <span>started {formatTimestamp(run.started_at)}</span>
          <span className="hidden md:inline">
            {run.ended_at ? ` · ended ${formatTimestamp(run.ended_at)}` : " · in flight"}
          </span>
        </div>
        <div className="flex min-w-0 items-center gap-2 text-xs text-muted-foreground">
          <Activity className="size-3.5" aria-hidden />
          {duration ? (
            <span
              className="font-mono tabular-nums"
              data-testid={`runs-list-duration-${run.run_id}`}
            >
              {duration}
            </span>
          ) : (
            <span className="font-mono">live</span>
          )}
        </div>
        <StatusBadge
          className="shrink-0"
          data-testid={`runs-list-status-${run.run_id}`}
          pulse={tone === "accent"}
          tone={tone}
        >
          {run.status}
        </StatusBadge>
      </div>
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
    <label className="flex flex-col gap-1 text-xs text-muted-foreground">
      <span className="eyebrow text-muted-foreground">{label}</span>
      <select
        className="rounded-[var(--radius-md)] border border-border bg-[color:var(--surface-inset)] px-2.5 py-1.5 text-sm text-foreground shadow-[var(--shadow-xs)] transition-[border-color,box-shadow] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/60"
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
