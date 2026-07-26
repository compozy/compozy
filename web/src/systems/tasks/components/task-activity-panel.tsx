import { Activity, AlertCircle } from "lucide-react";
import { useState } from "react";

import {
  Button,
  Empty,
  LiveBadge,
  PillGroup,
  Section,
  Timeline,
  type PillGroupItem,
} from "@agh/ui";

import {
  humanizeTaskEvent,
  matchesActivityFilter,
  TASK_ACTIVITY_FILTERS,
  type TaskActivityFilter,
} from "../lib/task-activity-copy";
import type { TaskTimelineItem } from "../types";
import { TaskActivityItem } from "./task-activity-item";
import { TaskRowsLoadingSkeleton } from "./task-loading-skeletons";

export interface TaskActivityPanelProps {
  items: readonly TaskTimelineItem[];
  isLoading?: boolean;
  errorMessage?: string | null;
  isLive?: boolean;
  onLoadMore?: () => void;
  canLoadMore?: boolean;
}

const FILTER_LABEL: Record<TaskActivityFilter, string> = {
  all: "All",
  runs: "Runs",
  reviews: "Reviews",
  changes: "Changes",
};

const FILTER_ITEMS: ReadonlyArray<PillGroupItem<TaskActivityFilter>> = TASK_ACTIVITY_FILTERS.map(
  value => ({
    value,
    label: FILTER_LABEL[value],
    testId: `tasks-activity-filter-${value}`,
  })
);

function emptyCopyFor(filter: TaskActivityFilter): { title: string; description: string } {
  if (filter === "reviews") {
    return {
      title: "No review activity yet",
      description: "Reviews appear here when a review policy is on.",
    };
  }
  return {
    title: "No activity recorded yet",
    description: "Events land here as this task moves through its runs.",
  };
}

/**
 * Activity tab: humanized newest-first feed with one filter group
 * (filtering replaces the old grouping modes) and the page's single LiveBadge
 * while the task stream is attached.
 *
 * @see docs/design/opendesign/tasks/TASK-DETAILS-REDESIGN-PLAN.md §4.6
 */
export function TaskActivityPanel({
  items,
  isLoading = false,
  errorMessage = null,
  isLive = false,
  onLoadMore,
  canLoadMore = false,
}: TaskActivityPanelProps) {
  const [filter, setFilter] = useState<TaskActivityFilter>("all");

  if (isLoading && items.length === 0) {
    return <TaskRowsLoadingSkeleton label="Loading activity" testId="tasks-activity-loading" />;
  }

  if (errorMessage && items.length === 0) {
    return (
      <Empty
        icon={AlertCircle}
        title="Couldn't load activity"
        description={errorMessage}
        data-testid="tasks-activity-error"
      />
    );
  }

  if (items.length === 0) {
    return (
      <Empty
        icon={Activity}
        title="No activity recorded yet"
        description="Events land here as this task moves through its runs."
        data-testid="tasks-activity-empty"
      />
    );
  }

  const visible: Array<{ item: TaskTimelineItem; view: ReturnType<typeof humanizeTaskEvent> }> = [];
  for (const item of items) {
    const view = humanizeTaskEvent(item);
    if (matchesActivityFilter(view, filter)) visible.push({ item, view });
  }
  const emptyCopy = emptyCopyFor(filter);

  return (
    <Section
      aria-label="Task activity"
      data-testid="tasks-activity-panel"
      right={isLive ? <LiveBadge data-testid="tasks-activity-live" /> : undefined}
      tabs={
        <PillGroup<TaskActivityFilter>
          aria-label="Filter activity"
          data-testid="tasks-activity-filters"
          items={FILTER_ITEMS}
          onChange={setFilter}
          size="sm"
          value={filter}
        />
      }
    >
      <div className="rounded-lg border border-line bg-canvas-soft px-4 pt-3">
        {visible.length === 0 ? (
          <p
            className="px-4 py-4 text-small-body text-muted"
            data-testid="tasks-activity-filter-empty"
          >
            {emptyCopy.title}. {emptyCopy.description}
          </p>
        ) : (
          <Timeline ariaLabel="Task activity events">
            {visible.map(({ item, view }) => (
              <TaskActivityItem isLive={isLive} item={item} key={item.event_id} view={view} />
            ))}
          </Timeline>
        )}
      </div>
      {canLoadMore && onLoadMore ? (
        <div className="flex items-center justify-center pt-1">
          <Button
            className="min-h-6"
            data-testid="tasks-activity-load-more"
            onClick={onLoadMore}
            size="sm"
            type="button"
            variant="neutral"
          >
            Load more
          </Button>
        </div>
      ) : null}
    </Section>
  );
}
