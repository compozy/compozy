import { useState } from "react";
import { AlertCircle, Lock, Search } from "lucide-react";
import {
  Alert,
  AlertDescription,
  cn,
  Empty,
  Metric,
  PAGE_CONTENT_GUTTER,
  Section,
  Skeleton,
  useTopbarSlot,
  type MetricTone,
} from "@compozy/ui";

import {
  automationScopeLabel,
  describeSchedule,
  formatDate,
  formatDateTime,
  formatRelativeTime,
} from "../lib/automation-formatters";
import { automationTargetLabel, projectAutomationTarget } from "../lib/automation-target";
import type { AutomationJob, AutomationRun, AutomationRunStatus } from "../types";
import { AutomationRunHistory } from "./automation-run-history";
import { AutomationDeleteAction } from "./automation-delete-action";
import {
  AutomationDetailActions,
  AutomationDetailOverflow,
} from "./automation-detail-topbar-actions";
import {
  AutomationTargetSection,
  JobAdvancedDetails,
  PromptSection,
} from "./automation-detail-sections";
import { AutomationEnableSwitch } from "./automation-enable-switch";

interface AutomationDetailState {
  isDeleting: boolean;
  isLoading: boolean;
  isTogglePending: boolean;
  isTriggerDisabled?: boolean;
  isTriggerPending: boolean;
}

interface AutomationDetailPanelProps {
  error: Error | null;
  state: AutomationDetailState;
  item: AutomationJob | undefined;
  onBack: () => void;
  onDelete: () => void | Promise<void>;
  onEdit: () => void;
  onToggleEnabled: (enabled: boolean) => void;
  onTriggerNow?: () => void;
  runs: AutomationRun[];
  runsError: Error | null;
  runsLoading: boolean;
}

interface JobMetricsCopy {
  successRateValue: string;
  successRateTone: MetricTone;
  lastRunValue: string;
  lastRunSubtext?: string;
  nextRunValue: string;
  nextRunSubtext?: string;
}

const TERMINAL_STATUSES: ReadonlySet<AutomationRunStatus> = new Set([
  "completed",
  "failed",
  "canceled",
]);

function computeJobMetrics(runs: AutomationRun[], job: AutomationJob): JobMetricsCopy {
  const terminal = runs.filter(run => TERMINAL_STATUSES.has(run.status));
  const completed = runs.filter(run => run.status === "completed").length;
  const lastCompleted = terminal.find(run => run.status === "completed" || run.status === "failed");

  let successRateValue = "—";
  let successRateTone: MetricTone = "default";
  if (terminal.length > 0) {
    const pct = (completed / terminal.length) * 100;
    successRateValue = `${Math.round(pct)}%`;
    successRateTone = pct >= 90 ? "success" : pct >= 70 ? "default" : "warning";
  }

  const lastRunValue = lastCompleted ? formatRelativeTime(lastCompleted.started_at) : "—";
  const lastRunSubtext = lastCompleted ? formatDateTime(lastCompleted.started_at) : undefined;

  const nextRun = job.scheduler?.next_run_at ?? job.next_run;
  const nextRunValue = formatRelativeTime(nextRun);
  const nextRunSubtext = nextRun ? formatDateTime(nextRun) : undefined;

  return {
    successRateValue,
    successRateTone,
    lastRunValue,
    lastRunSubtext,
    nextRunValue,
    nextRunSubtext,
  };
}

function JobStatsSection({ job, runs }: { job: AutomationJob; runs: AutomationRun[] }) {
  const metrics = computeJobMetrics(runs, job);
  return (
    <Section label="At a glance">
      <div className="grid gap-3 @md:grid-cols-3">
        <Metric
          data-testid="automation-job-metric-next-run"
          label="Next run"
          subtext={metrics.nextRunSubtext}
          value={metrics.nextRunValue}
        />
        <Metric
          data-testid="automation-job-metric-last-run"
          label="Last run"
          subtext={metrics.lastRunSubtext}
          value={metrics.lastRunValue}
        />
        <Metric
          data-testid="automation-job-metric-success-rate"
          label="Recent success"
          subtext="of recent runs"
          tone={metrics.successRateTone}
          value={metrics.successRateValue}
        />
      </div>
    </Section>
  );
}

/** Jobs and triggers use separate detail surfaces; trigger details use `TriggerDetailPanel`. */
export function AutomationDetailPanel({
  error,
  state,
  item,
  onBack,
  onDelete,
  onEdit,
  onToggleEnabled,
  onTriggerNow,
  runs,
  runsError,
  runsLoading,
}: AutomationDetailPanelProps) {
  const {
    isDeleting,
    isLoading,
    isTogglePending,
    isTriggerDisabled = false,
    isTriggerPending,
  } = state;
  if (isLoading) {
    return <AutomationDetailSkeleton />;
  }

  if (error) {
    return (
      <div
        className="flex min-h-0 flex-1 items-center justify-center py-10"
        data-testid="automation-detail-error"
      >
        <Empty
          className="max-w-md"
          description={error.message ?? "Failed to load automation details"}
          icon={AlertCircle}
          title="Unable to load details"
        />
      </div>
    );
  }

  if (!item) {
    return (
      <div
        className="flex min-h-0 flex-1 items-center justify-center py-10"
        data-testid="automation-detail-empty"
      >
        <Empty
          className="max-w-md"
          description="This job is no longer available."
          icon={Search}
          title="Job unavailable"
        />
      </div>
    );
  }

  return (
    <AutomationDetailLoadedPanel
      item={item}
      onBack={onBack}
      onDelete={onDelete}
      onEdit={onEdit}
      onToggleEnabled={onToggleEnabled}
      onTriggerNow={onTriggerNow}
      runs={runs}
      runsError={runsError}
      runsLoading={runsLoading}
      state={{ isDeleting, isTogglePending, isTriggerDisabled, isTriggerPending }}
    />
  );
}

function AutomationDetailSkeleton() {
  return (
    <div
      aria-busy="true"
      className={cn(PAGE_CONTENT_GUTTER, "flex min-h-0 flex-1 flex-col overflow-hidden")}
      data-testid="automation-detail-loading"
      role="status"
    >
      <div className="flex flex-col gap-2 pt-4">
        <div className="flex gap-2">
          <Skeleton className="h-5 w-20" />
          <Skeleton className="h-5 w-24" />
        </div>
        <Skeleton className="h-3 w-72" />
      </div>
      <div className="min-h-0 flex-1 space-y-6 overflow-hidden py-5">
        {[0, 1, 2, 3].map(index => (
          <div className="space-y-2.5" key={index}>
            <Skeleton className="h-3 w-24" />
            <Skeleton className={index === 1 ? "h-28 w-full" : "h-20 w-full"} />
          </div>
        ))}
      </div>
      <span className="sr-only">Loading automation details</span>
    </div>
  );
}

interface AutomationDetailLoadedPanelProps {
  item: AutomationJob;
  onBack: () => void;
  onDelete: () => void | Promise<void>;
  onEdit: () => void;
  onToggleEnabled: (enabled: boolean) => void;
  onTriggerNow?: () => void;
  runs: AutomationRun[];
  runsError: Error | null;
  runsLoading: boolean;
  state: Omit<Required<AutomationDetailState>, "isLoading">;
}

function AutomationDetailLoadedPanel({
  item,
  onBack,
  onDelete,
  onEdit,
  onToggleEnabled,
  onTriggerNow,
  runs,
  runsError,
  runsLoading,
  state,
}: AutomationDetailLoadedPanelProps) {
  const { isDeleting, isTogglePending, isTriggerDisabled, isTriggerPending } = state;
  const isDynamic = item.source === "dynamic";
  const target = projectAutomationTarget(item);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const showRunNow = Boolean(onTriggerNow);
  const detailActions = showRunNow ? (
    <AutomationDetailActions
      onTriggerNow={onTriggerNow}
      triggerDisabled={isTriggerDisabled}
      triggerPending={isTriggerPending}
    />
  ) : undefined;
  const detailOverflow = isDynamic ? (
    <AutomationDetailOverflow onDelete={() => setDeleteOpen(true)} onEdit={onEdit} />
  ) : undefined;
  useTopbarSlot({
    onBack,
    crumbs: [{ id: "catalog", label: "Jobs", onSelect: onBack }],
    crumb: item.name,
    actions: detailActions,
    overflow: detailOverflow,
  });

  return (
    <section
      className={cn(PAGE_CONTENT_GUTTER, "flex min-h-0 flex-1 flex-col overflow-hidden")}
      data-testid="automation-detail-panel"
    >
      {isDynamic ? (
        <AutomationDeleteAction
          hideTrigger
          isPending={isDeleting}
          kind="jobs"
          name={item.name}
          onConfirm={onDelete}
          onOpenChange={setDeleteOpen}
          open={deleteOpen}
        />
      ) : null}
      <div
        className="mt-4 mb-5 flex items-start justify-between gap-6 border-b border-line pb-4"
        data-testid="automation-detail-header"
      >
        <div className="min-w-0 flex-1">
          <p
            className="text-pretty text-item-title font-medium text-fg-strong"
            data-testid="automation-detail-schedule"
          >
            {describeSchedule(item.schedule)}
          </p>
          <p className="mt-1 text-form-label text-subtle" data-testid="automation-detail-meta">
            {`${automationTargetLabel(target)} · ${automationScopeLabel(item.scope)} · Updated ${formatDate(item.updated_at)}`}
          </p>
        </div>
        <AutomationEnableSwitch
          enabled={item.enabled}
          onEnabledChange={onToggleEnabled}
          pending={isTogglePending}
          labelTestId="job-enable-label"
          switchTestId="toggle-automation-btn"
        />
      </div>

      <div className="@container min-h-0 flex-1 space-y-6 overflow-y-auto pb-16">
        {!isDynamic ? (
          <Alert data-testid="automation-detail-lock" variant="neutral">
            <Lock aria-hidden="true" />
            <AlertDescription>
              {item.source === "config"
                ? "This job is defined in configuration files. You can only turn it on or off here."
                : "This job comes from an installed package. You can only turn it on or off here."}
            </AlertDescription>
          </Alert>
        ) : null}

        <JobStatsSection job={item} runs={runs} />

        {target.kind === "loop" ? <AutomationTargetSection target={target} /> : null}
        {target.kind === "agent" ? <PromptSection prompt={target.prompt} /> : null}

        <AutomationRunHistory
          emptyDescription="Runs show up here after the job runs for the first time."
          emptyTitle="No runs recorded yet"
          error={runsError}
          isLoading={runsLoading}
          loopWorkspaceId={target.kind === "loop" ? target.workspaceId : undefined}
          runs={runs}
          title="Runs"
        />

        <JobAdvancedDetails job={item} />
      </div>
    </section>
  );
}
