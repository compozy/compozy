import { ChevronRight } from "lucide-react";
import type { ComponentPropsWithoutRef } from "react";

import { Button, cn, DescriptionCard, Section } from "@agh/ui";

import type { TaskDetailView, TaskRun, TaskTimelineItem } from "../types";
import { latestTaskRun } from "../lib/task-run-presentation";
import { TaskActivityItem } from "./task-activity-item";
import { TaskDependenciesSection } from "./task-dependencies-section";
import { TaskNowStrip, type TaskNowStripHandlers } from "./task-now-strip";
import { TaskResultSection } from "./task-result-section";
import { TaskSubtasksSection } from "./task-subtasks-section";

export const TASK_RESULT_ANCHOR_ID = "tasks-detail-result";

const RECENT_ACTIVITY_LIMIT = 5;

export interface TaskOverviewPanelProps extends ComponentPropsWithoutRef<"div"> {
  detail: TaskDetailView;
  runs: readonly TaskRun[];
  timeline: readonly TaskTimelineItem[];
  isLive: boolean;
  nowHandlers: TaskNowStripHandlers;
  nowPending?: Parameters<typeof TaskNowStrip>[0]["pending"];
  onViewAllActivity: () => void;
}

function CompletedTaskResult({ runs }: { runs: readonly TaskRun[] }) {
  const lastCompleted = latestTaskRun(runs, "completed");
  if (!lastCompleted) return null;

  return (
    <TaskResultSection
      data-testid="tasks-detail-result-section"
      emptyMessage="No result was recorded for the completed run."
      id={TASK_RESULT_ANCHOR_ID}
      jsonTestId="tasks-detail-result-json"
      result={lastCompleted.result}
    />
  );
}

/**
 * Overview tab: Now strip → result (terminal) → description → subtasks
 * → dependencies → recent activity. Sections self-suppress when empty so a
 * simple task reads as a single quiet column.
 *
 * @see docs/design/opendesign/tasks/TASK-DETAILS-REDESIGN-PLAN.md §4.3
 */
export function TaskOverviewPanel({
  detail,
  runs,
  timeline,
  isLive,
  nowHandlers,
  nowPending,
  onViewAllActivity,
  className,
  ...props
}: TaskOverviewPanelProps) {
  const record = detail.task;
  const children = detail.children ?? [];
  const dependencies = detail.dependency_references ?? [];
  const recent = timeline.slice(0, RECENT_ACTIVITY_LIMIT);
  const description = record.description?.trim();

  return (
    <div
      {...props}
      className={cn("flex flex-col gap-6", className)}
      data-testid="tasks-detail-overview"
    >
      <TaskNowStrip detail={detail} handlers={nowHandlers} pending={nowPending} runs={runs} />

      {record.status === "completed" ? <CompletedTaskResult runs={runs} /> : null}

      <Section data-testid="tasks-detail-description" label="Description">
        {description ? (
          <DescriptionCard className="border border-line px-4 py-3.5">
            {description}
          </DescriptionCard>
        ) : (
          <p className="text-small-body text-subtle">No description.</p>
        )}
      </Section>

      <TaskSubtasksSection items={children} />
      <TaskDependenciesSection dependencies={dependencies} />

      {recent.length > 0 ? (
        <Section
          data-testid="tasks-detail-recent-activity"
          label="Recent activity"
          right={
            <Button
              className="-mr-1.5 min-h-6 px-1.5 py-0.5 text-eyebrow font-medium text-muted"
              data-testid="tasks-detail-view-all-activity"
              onClick={onViewAllActivity}
              size="sm"
              type="button"
              variant="ghost"
            >
              View all
              <ChevronRight aria-hidden="true" className="size-3" />
            </Button>
          }
        >
          <ul
            aria-label="Recent task activity"
            className="flex flex-col rounded-lg border border-line bg-canvas-soft px-4 pt-3"
          >
            {recent.map(item => (
              <TaskActivityItem
                className="pl-0"
                hasMarker={false}
                isLive={isLive}
                item={item}
                key={item.event_id}
              />
            ))}
          </ul>
        </Section>
      ) : null}
    </div>
  );
}
