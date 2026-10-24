import { Activity } from "lucide-react";

import { Empty, LiveBadge, Section, SkeletonRegion, Timeline } from "@compozy/ui";

import { taskRunTimelineItems } from "../lib/task-run-presentation";
import type { TaskTimelineItem } from "../types";
import { TaskActivityItem } from "./task-activity-item";
import { TaskTimelineSkeleton } from "./task-loading-skeletons";
import { TASK_TIMELINE_CARD_CLASS } from "./task-surface-classes";

export interface TaskRunActivitySectionProps {
  errorMessage?: string | null;
  isLive: boolean;
  isLoading?: boolean;
  runId: string;
  timeline: readonly TaskTimelineItem[];
}

export function TaskRunActivitySection({
  errorMessage = null,
  isLive,
  isLoading = false,
  runId,
  timeline,
}: TaskRunActivitySectionProps) {
  const items = taskRunTimelineItems(timeline, runId);

  return (
    <Section
      data-testid="tasks-run-activity"
      label="Run activity"
      right={isLive ? <LiveBadge data-testid="tasks-run-activity-live" /> : undefined}
    >
      {isLoading ? (
        <SkeletonRegion
          className={TASK_TIMELINE_CARD_CLASS}
          data-testid="tasks-run-activity-loading"
          label="Loading run activity"
        >
          <TaskTimelineSkeleton events={3} />
        </SkeletonRegion>
      ) : errorMessage ? (
        <p className="text-small-body text-danger" role="alert">
          {errorMessage}
        </p>
      ) : (
        <div className={TASK_TIMELINE_CARD_CLASS}>
          {items.length === 0 ? (
            <Empty
              className="pt-2 pb-5"
              icon={Activity}
              size="compact"
              title="No events recorded for this attempt yet."
            />
          ) : (
            <Timeline ariaLabel="Run activity events">
              {items.map(item => (
                <TaskActivityItem isLive={isLive} item={item} key={item.event_id} />
              ))}
            </Timeline>
          )}
        </div>
      )}
    </Section>
  );
}
