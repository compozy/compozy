import { LiveBadge, Section, Skeleton } from "@agh/ui";

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
      ) : items.length === 0 ? (
        <p className="rounded-lg border border-line bg-canvas-soft px-4 py-3.5 text-small-body text-muted">
          No events recorded for this attempt yet.
        </p>
      ) : (
        <div className="overflow-hidden rounded-lg border border-line bg-canvas-soft">
          {items.map(item => (
            <TaskActivityItem isLive={isLive} item={item} key={item.event_id} />
          ))}
        </div>
      )}
    </Section>
  );
}
