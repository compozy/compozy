import { LiveBadge, Section, Skeleton, Timeline } from "@compozy/ui";

import { taskRunTimelineItems } from "../lib/task-run-presentation";
import type { TaskTimelineItem } from "../types";
import { TaskActivityItem } from "./task-activity-item";

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
        <Skeleton className="h-20 rounded-lg" />
      ) : errorMessage ? (
        <p className="text-small-body text-danger" role="alert">
          {errorMessage}
        </p>
      ) : (
        <div className="rounded-lg border border-line bg-canvas-soft px-4 pt-3">
          {items.length === 0 ? (
            <p className="px-4 py-4 text-small-body text-muted">
              No events recorded for this attempt yet.
            </p>
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
