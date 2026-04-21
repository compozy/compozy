import type { ReactElement } from "react";

import {
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
        {isRefetching ? (
          <span className="text-xs text-muted-foreground" data-testid="runs-list-refreshing">
            refreshing…
          </span>
        ) : null}
      </div>

      {degradedReason ? (
        <p
          className="rounded-[var(--radius-md)] border border-[color:var(--color-warning)] bg-black/10 px-4 py-3 text-sm text-[color:var(--color-warning)]"
          data-testid="runs-list-degraded"
          role="status"
        >
          {degradedReason}
        </p>
      ) : null}

      {error ? (
        <p
          className="rounded-[var(--radius-md)] border border-[color:var(--color-danger)] bg-black/20 px-4 py-3 text-sm text-[color:var(--color-danger)]"
          data-testid="runs-list-error"
          role="alert"
        >
          {error}
        </p>
      ) : null}

      {isLoading ? (
        <p className="text-sm text-muted-foreground" data-testid="runs-list-loading">
          Loading runs…
        </p>
      ) : null}

      {!isLoading && runs.length === 0 && !error ? (
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

      {runs.length > 0 ? (
        <ul className="grid gap-3" data-testid="runs-list-items">
          {runs.map(run => (
            <RunRow key={run.run_id} run={run} />
          ))}
        </ul>
      ) : null}
    </div>
  );
}

function RunRow({ run }: { run: Run }): ReactElement {
  const tone = resolveStatusTone(run.status);
  return (
    <li>
      <SurfaceCard data-testid={`runs-list-row-${run.run_id}`}>
        <SurfaceCardHeader>
          <div>
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
            </SurfaceCardDescription>
          </div>
          <StatusBadge data-testid={`runs-list-status-${run.run_id}`} tone={tone}>
            {run.status}
          </StatusBadge>
        </SurfaceCardHeader>
        {run.error_text ? (
          <SurfaceCardBody>
            <p
              className="text-sm text-[color:var(--color-danger)]"
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
      <span className="font-disket uppercase tracking-[0.14em]">{label}</span>
      <select
        className="rounded-[var(--radius-sm)] border border-border bg-transparent px-2 py-1 text-sm text-foreground focus:outline-none"
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
