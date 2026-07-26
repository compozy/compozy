import { Link } from "@tanstack/react-router";
import { Search } from "lucide-react";
import type { ComponentPropsWithoutRef, ReactNode } from "react";

import { Button, cn, MonoId, OwnerAvatar, Pill, PropertyRow, Spinner, Time } from "@agh/ui";

import {
  computeElapsed,
  ownerAvatarKindFor,
  taskOwnerLabel,
  taskRunStatusLabel,
  taskRunStatusTone,
} from "../lib/task-formatters";
import {
  taskExecutionProfileSummary,
  taskPropertiesRunSummary,
} from "../lib/task-properties-presentation";
import type { TaskDetailView, TaskExecutionProfile, TaskPriority, TaskRun } from "../types";
import { TaskAutoEnqueueSwitch, TaskPriorityEditor } from "./task-rail-editors";

export interface TaskPropertiesRailProps extends ComponentPropsWithoutRef<"div"> {
  detail: TaskDetailView;
  runs: readonly TaskRun[];
  profile?: TaskExecutionProfile | null;
  onEditSetup: () => void;
  onInspect: () => void;
  onApprove: () => void;
  onReject: () => void;
  approvalPending?: { approve?: boolean; reject?: boolean };
  updatePending?: boolean;
  onPriorityChange: (priority: TaskPriority) => void;
  onAutoEnqueueChange: (enabled: boolean) => void;
}

function RailSection({
  label,
  action,
  children,
}: {
  label: string;
  action?: ReactNode;
  children: ReactNode;
}) {
  return (
    <section className="border-t border-line-soft px-4 py-3.5 first:border-t-0">
      <header className="mb-2 flex items-center justify-between gap-2">
        <h3 className="eyebrow text-subtle">{label}</h3>
        {action}
      </header>
      {children}
    </section>
  );
}

/**
 * 320px properties rail: tier a/b fields only, grouped, with the sole
 * operator entry point (Inspect) in the footer. No lease, heartbeat, claim
 * hash, or seq here — those stay behind Inspect.
 *
 * @see docs/design/opendesign/tasks/TASK-DETAILS-REDESIGN-PLAN.md §4.4
 */
export function TaskPropertiesRail({
  detail,
  runs,
  profile,
  onEditSetup,
  onInspect,
  onApprove,
  onReject,
  approvalPending = {},
  updatePending = false,
  onPriorityChange,
  onAutoEnqueueChange,
  className,
  ...props
}: TaskPropertiesRailProps) {
  const record = detail.task;
  const activeRun = detail.summary?.active_run ?? null;
  const owner = record.owner ?? null;
  const ownerName = owner ? taskOwnerLabel(owner) : "Unassigned";
  const { worker, model, sandbox, channel } = taskExecutionProfileSummary(profile);
  const { attemptsLabel, lastFailedRun, stuckRun } = taskPropertiesRunSummary(detail, runs);
  const approvalBusy = Boolean(approvalPending.approve || approvalPending.reject);

  return (
    <div
      {...props}
      className={cn("overflow-hidden rounded-lg border border-line bg-canvas-soft", className)}
      data-testid="tasks-detail-rail"
    >
      {record.approval_state === "pending" ? (
        <RailSection label="Approval">
          <PropertyRow label="State">
            <Pill.Dot tone="info" />
            Pending
          </PropertyRow>
          <PropertyRow label="Requested by">{record.created_by?.ref ?? "unknown"}</PropertyRow>
          <div className="mt-2 flex items-center justify-end gap-2">
            <Button
              aria-busy={approvalPending.reject || undefined}
              className="min-h-6"
              data-testid="tasks-rail-reject"
              disabled={approvalBusy}
              onClick={onReject}
              size="sm"
              type="button"
              variant="ghost"
            >
              {approvalPending.reject ? <Spinner aria-hidden="true" className="size-3" /> : null}
              {approvalPending.reject ? "Rejecting…" : "Reject"}
            </Button>
            <Button
              aria-busy={approvalPending.approve || undefined}
              className="min-h-6"
              data-testid="tasks-rail-approve"
              disabled={approvalBusy}
              onClick={onApprove}
              size="sm"
              type="button"
              variant="neutral"
            >
              {approvalPending.approve ? <Spinner aria-hidden="true" className="size-3" /> : null}
              {approvalPending.approve ? "Approving…" : "Approve"}
            </Button>
          </div>
        </RailSection>
      ) : null}

      {stuckRun ? (
        <RailSection label="Current run">
          <PropertyRow label="Status">
            <Pill.Dot tone={taskRunStatusTone(stuckRun.status)} />
            {taskRunStatusLabel(stuckRun.status)}
          </PropertyRow>
          {stuckRun.claimed_by?.ref ? (
            <PropertyRow label="Claimed by">
              <OwnerAvatar
                name={stuckRun.claimed_by.ref}
                ownerId={stuckRun.claimed_by.ref}
                ownerKind={ownerAvatarKindFor(stuckRun.claimed_by.kind)}
                size="sm"
              />
              <span className="truncate">{stuckRun.claimed_by.ref}</span>
            </PropertyRow>
          ) : null}
          <PropertyRow label="Run id" mono>
            <MonoId value={stuckRun.id} />
          </PropertyRow>
        </RailSection>
      ) : null}

      {lastFailedRun ? (
        <RailSection label="Last run">
          <PropertyRow label="Status">
            <Pill.Dot tone="danger" />
            Failed
          </PropertyRow>
          {lastFailedRun.ended_at ? (
            <PropertyRow label="Ended">
              <Time iso={lastFailedRun.ended_at} mode="relative" />
            </PropertyRow>
          ) : null}
          <PropertyRow label="Duration" mono>
            {computeElapsed(lastFailedRun) ?? "—"}
          </PropertyRow>
          <PropertyRow label="Run id" mono>
            <MonoId value={lastFailedRun.id} />
          </PropertyRow>
        </RailSection>
      ) : null}

      <RailSection label="Properties">
        <PropertyRow
          editor={
            <TaskPriorityEditor
              onChange={onPriorityChange}
              pending={updatePending}
              priority={record.priority ?? "medium"}
            />
          }
          label="Priority"
        />
        <PropertyRow label="Owner">
          {owner ? (
            <>
              <OwnerAvatar
                name={ownerName}
                ownerId={owner.ref ?? ownerName}
                ownerKind={ownerAvatarKindFor(owner.kind)}
                size="sm"
              />
              <span className="truncate">{ownerName}</span>
            </>
          ) : (
            <span className="text-muted">Unassigned</span>
          )}
        </PropertyRow>
        {record.workspace_id ? (
          <PropertyRow label="Workspace">{record.workspace_id}</PropertyRow>
        ) : null}
        {record.parent_task_id ? (
          <PropertyRow
            editor={
              <Link
                className="inline-flex min-h-6 min-w-0 items-center rounded-sm px-1.5 py-0.5 text-small-body font-medium text-fg hover:bg-row-hover focus-visible:outline-none focus-visible:shadow-focus-ring"
                data-testid="tasks-rail-parent"
                params={{ id: record.parent_task_id }}
                to="/tasks/$id"
              >
                <span className="truncate">Open parent task</span>
              </Link>
            }
            label="Parent task"
          />
        ) : null}
      </RailSection>

      <RailSection
        action={
          <Button
            className="-mr-1.5 min-h-6 px-1.5 py-0.5 text-eyebrow font-medium text-muted"
            data-testid="tasks-rail-edit-setup"
            onClick={onEditSetup}
            size="sm"
            type="button"
            variant="ghost"
          >
            Edit setup
          </Button>
        }
        label="Execution"
      >
        {worker ? <PropertyRow label="Agent">{worker}</PropertyRow> : null}
        {model ? (
          <PropertyRow label="Model" mono>
            {model}
          </PropertyRow>
        ) : null}
        {sandbox ? <PropertyRow label="Sandbox">{sandbox}</PropertyRow> : null}
        <PropertyRow label="Attempts">{attemptsLabel}</PropertyRow>
        <PropertyRow
          editor={
            <TaskAutoEnqueueSwitch
              enabled={Boolean(record.auto_enqueue_on_ready)}
              onChange={onAutoEnqueueChange}
              pending={updatePending}
            />
          }
          label="Auto-enqueue"
        />
        {channel ? (
          <PropertyRow label="Channel" mono>
            {channel}
          </PropertyRow>
        ) : null}
      </RailSection>

      <RailSection label="Activity">
        <PropertyRow label="Created">
          <Time iso={record.created_at} mode="relative" />
        </PropertyRow>
        <PropertyRow label="Updated">
          <Time iso={record.updated_at} mode="relative" />
        </PropertyRow>
        {record.closed_at ? (
          <PropertyRow label="Closed">
            <Time iso={record.closed_at} mode="relative" />
          </PropertyRow>
        ) : null}
        <PropertyRow label="Created by">{record.created_by?.ref ?? "unknown"}</PropertyRow>
        <PropertyRow label="Task id" mono>
          <MonoId value={record.id} />
        </PropertyRow>
        {activeRun ? (
          <PropertyRow label="Run id" mono>
            <MonoId value={activeRun.id} />
          </PropertyRow>
        ) : null}
      </RailSection>

      <footer className="flex items-center justify-between gap-2 border-t border-line-soft px-3 py-2.5">
        <Button
          className="min-h-6"
          data-testid="tasks-rail-inspect"
          onClick={onInspect}
          size="sm"
          type="button"
          variant="ghost"
        >
          <Search aria-hidden="true" className="size-3" />
          Inspect
        </Button>
        <span className="truncate font-mono text-micro text-faint">
          agh task inspect {record.id}
        </span>
      </footer>
    </div>
  );
}
