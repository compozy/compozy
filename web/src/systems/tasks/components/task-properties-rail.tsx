import { Link } from "@tanstack/react-router";
import { Search } from "@compozy/ui/icons";
import type { ComponentPropsWithoutRef } from "react";

import {
  Button,
  cn,
  OwnerAvatar,
  PropertyRow,
  PropertyRowSkeleton,
  SkeletonRegion,
  SkeletonText,
  StateGlyph,
  Time,
} from "@compozy/ui";

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
import { TASK_RAIL_CLASS, TASK_RAIL_FOOTER_CLASS } from "./task-surface-classes";

export interface TaskPropertiesRailProps extends ComponentPropsWithoutRef<"div"> {
  detail: TaskDetailView;
  runs: readonly TaskRun[];
  profile?: TaskExecutionProfile | null;
  /** The execution profile is still being read: its agent and model rows are placeholders. */
  profileLoading?: boolean;
  /** The run history is still being read: what derives from it is a placeholder. */
  runsLoading?: boolean;
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
  profileLoading = false,
  runsLoading = false,
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
    <div {...props} className={cn(TASK_RAIL_CLASS, className)} data-testid="tasks-detail-rail">
      {/* Leads the rail: "what is this record, which run owns it" comes first. */}
      {record.loop ? <TaskLoopProvenance loop={record.loop} /> : null}

      <TaskRunHistorySections
        // A failed task with no live run gets a "Last run" band once its runs arrive.
        lastRunPending={runsLoading && !lastFailedRun && record.status === "failed"}
        lastFailedRun={lastFailedRun}
        stuckRun={stuckRun}
      />

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
            <span className="text-muted-foreground">Unassigned</span>
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

      <TaskExecutionSection
        attemptsLabel={attemptsLabel}
        autoEnqueue={Boolean(record.auto_enqueue_on_ready)}
        model={model}
        onAutoEnqueueChange={onAutoEnqueueChange}
        onEditSetup={onEditSetup}
        profileLoading={profileLoading}
        runsLoading={runsLoading}
        updatePending={updatePending}
        worker={worker}
      />

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

      <footer className={TASK_RAIL_FOOTER_CLASS}>
        <Button
          data-testid="tasks-rail-inspect"
          onClick={onInspect}
          size="sm"
          type="button"
          variant="ghost"
        >
          <Search aria-hidden="true" data-icon="inline-start" />
          Inspect
        </Button>
      </footer>
    </div>
  );
}

interface TaskExecutionSectionProps {
  worker: string | null;
  model: string | null;
  attemptsLabel: string;
  autoEnqueue: boolean;
  profileLoading: boolean;
  runsLoading: boolean;
  updatePending: boolean;
  onEditSetup: () => void;
  onAutoEnqueueChange: (enabled: boolean) => void;
}

/** Who works on the task and how it starts. The agent and model rows wait on the profile read. */
function TaskExecutionSection({
  worker,
  model,
  attemptsLabel,
  autoEnqueue,
  profileLoading,
  runsLoading,
  updatePending,
  onEditSetup,
  onAutoEnqueueChange,
}: TaskExecutionSectionProps) {
  return (
    <RailSection
      action={
        <Button
          // Optical: the link's text lands on the rail's value edge.
          className="-mr-2.5"
          data-testid="tasks-rail-edit-setup"
          onClick={onEditSetup}
          size="sm"
          type="button"
          variant="link"
        >
          Edit setup
        </Button>
      }
      label="Execution"
    >
      {profileLoading ? (
        <SkeletonRegion label="Loading setup">
          <PropertyRowSkeleton count={2} />
        </SkeletonRegion>
      ) : (
        <>
          {worker ? <PropertyRow label="Agent">{worker}</PropertyRow> : null}
          {model ? (
            <PropertyRow label="Model" mono>
              {model}
            </PropertyRow>
          ) : null}
        </>
      )}

      <PropertyRow label="Attempts">
        {runsLoading ? <SkeletonText className="w-10" /> : attemptsLabel}
      </PropertyRow>
      <PropertyRow
        editor={
          <TaskAutoEnqueueSwitch
            enabled={autoEnqueue}
            onChange={onAutoEnqueueChange}
            pending={updatePending}
          />
        }
        label="Start automatically"
      />
    </RailSection>
  );
}

function TaskRunHistorySections({
  stuckRun,
  lastFailedRun,
  lastRunPending,
}: Pick<ReturnType<typeof taskPropertiesRunSummary>, "stuckRun" | "lastFailedRun"> & {
  lastRunPending: boolean;
}) {
  return (
    <>
      {lastRunPending ? (
        <RailSection label="Last run">
          <SkeletonRegion label="Loading last run">
            <PropertyRowSkeleton count={3} />
          </SkeletonRegion>
        </RailSection>
      ) : null}

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
