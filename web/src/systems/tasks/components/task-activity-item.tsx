import { Time, TimelineEvent, type TimelineEventProps } from "@compozy/ui";

import { humanizeTaskEvent, type TaskActivityView } from "../lib/task-activity-copy";
import { resolveEventTone, visualFor } from "../lib/timeline-visuals";
import type { TaskTimelineItem } from "../types";

/**
 * One humanized activity row: plain-language title first, optional detail,
 * and freshness right-aligned. Raw event types stay in the Inspect drawer.
 */
export interface TaskActivityItemProps extends Omit<
  TimelineEventProps,
  "description" | "meta" | "time" | "title" | "tone"
> {
  item: TaskTimelineItem;
  isLive: boolean;
  view?: TaskActivityView;
}

export function TaskActivityItem({
  item,
  isLive,
  view = humanizeTaskEvent(item),
  className,
  ...props
}: TaskActivityItemProps) {
  const tone = resolveEventTone(item.event_type, isLive);

  return (
    <TimelineEvent
      {...props}
      className={className}
      data-category={view.category}
      data-testid={`tasks-activity-item-${item.event_id}`}
      description={view.detail}
      icon={visualFor(item.event_type).icon}
      title={view.title}
      time={item.timestamp ? <Time iso={item.timestamp} mode="relative" /> : undefined}
      tone={tone}
    />
  );
}
