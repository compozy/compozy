import { Link } from "@tanstack/react-router";
import { Search } from "lucide-react";
import type { ComponentPropsWithoutRef } from "react";

import { Button, cn, OwnerAvatar, PropertyRow, StateGlyph, Time } from "@compozy/ui";

import {
  computeElapsed,
  ownerAvatarKindFor,
  taskOwnerLabel,
  taskRunStatusLabel,
  taskRunStateGlyph,
} from "../lib/task-formatters";
import {
  taskExecutionProfileSummary,
  taskPropertiesRunSummary,
} from "../lib/task-properties-presentation";
import type { TaskDetailView, TaskExecutionProfile, TaskPriority, TaskRun } from "../types";
import { TaskLoopProvenance } from "./task-loop-provenance";
import { TaskAutoEnqueueSwitch, TaskPriorityEditor } from "./task-rail-editors";
import { TaskRailSection as RailSection } from "./task-rail-section";

export interface TaskPropertiesRailProps extends ComponentPropsWithoutRef<"div"> {
  detail: TaskDetailView;
  runs: readonly TaskRun[];
  profile?: TaskExecutionProfile | null;
  onEditSetup: () => void;
  onInspect: () => void;
  updatePending?: boolean;
  onPriorityChange: (priority: TaskPriority) => void;
  onAutoEnqueueChange: (enabled: boolean) => void;
}

/**
 * 320px properties rail: tier a/b fields only, grouped, with the sole
 * operator entry point (Inspect) in the footer. Ids, lease, heartbeat, claim
 * hash, and seq stay behind Inspect; approval lives in the window head.
 *
 * @see docs/design/opendesign/tasks/TASK-DETAILS-REDESIGN-PLAN.md §4.4
 */
export function TaskPropertiesRail({
  detail,
  runs,
  profile,
  onEditSetup,
  onInspect,
  updatePending = false,
  onPriorityChange,
  onAutoEnqueueChange,
  className,
  ...props
}: TaskPropertiesRailProps) {
  const record = detail.task;
  const owner = record.owner ?? null;
  const ownerName = owner ? taskOwnerLabel(owner) : "Unassigned";
  const { worker, model } = taskExecutionProfileSummary(profile);
  const { attemptsLabel, lastFailedRun, stuckRun } = taskPropertiesRunSummary(detail, runs);

  return (
    <div
      {...props}
      className={cn("overflow-hidden rounded-lg bg-canvas shadow-card", className)}
      data-testid="tasks-detail-rail"
    >
      {/* Leads the rail: "what is this record, which run owns it" comes first. */}
      {record.loop ? <TaskLoopProvenance loop={record.loop} /> : null}

      <TaskRunHistorySections stuckRun={stuckRun} lastFailedRun={lastFailedRun} />

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
        {record.parent_task_id ? (
          <PropertyRow
            editor={
              <Link
                className="inline-flex min-h-6 min-w-0 items-center rounded-sm px-1.5 py-0.5 text-small-body font-medium text-fg hover:bg-surface-2 focus-visible:outline-none focus-visible:shadow-focus-ring"
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

        <PropertyRow label="Attempts">{attemptsLabel}</PropertyRow>
        <PropertyRow
          editor={
            <TaskAutoEnqueueSwitch
              enabled={Boolean(record.auto_enqueue_on_ready)}
              onChange={onAutoEnqueueChange}
              pending={updatePending}
            />
          }
          label="Start automatically"
        />
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
      </RailSection>

      <footer className="flex items-center gap-2 border-t border-line-soft px-3 py-2.5">
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
      </footer>
    </div>
  );
}

function TaskRunHistorySections({
  stuckRun,
  lastFailedRun,
}: Pick<ReturnType<typeof taskPropertiesRunSummary>, "stuckRun" | "lastFailedRun">) {
  return (
    <>
      {stuckRun ? (
        <RailSection label="Current run">
          <PropertyRow label="Status">
            <StateGlyph state={taskRunStateGlyph(stuckRun.status)} />
            {taskRunStatusLabel(stuckRun.status)}
          </PropertyRow>
          {stuckRun.claimed_by?.ref ? (
            <PropertyRow label="Worked on by">
              <OwnerAvatar
                name={stuckRun.claimed_by.ref}
                ownerId={stuckRun.claimed_by.ref}
                ownerKind={ownerAvatarKindFor(stuckRun.claimed_by.kind)}
                size="sm"
              />
              <span className="truncate">{stuckRun.claimed_by.ref}</span>
            </PropertyRow>
          ) : null}
        </RailSection>
      ) : null}

      {lastFailedRun ? (
        <RailSection label="Last run">
          <PropertyRow label="Status">
            <StateGlyph state="failed" />
            Failed
          </PropertyRow>
          {lastFailedRun.ended_at ? (
            <PropertyRow label="Ended">
              <Time iso={lastFailedRun.ended_at} mode="relative" />
            </PropertyRow>
          ) : null}
          <PropertyRow label="Duration">
            <span className="tabular-nums" data-testid="task-last-run-duration">
              {computeElapsed(lastFailedRun) ?? "—"}
            </span>
          </PropertyRow>
        </RailSection>
      ) : null}
    </>
  );
}
