import { Time, TimelineEvent, type TimelineEventProps } from "@agh/ui";

import { humanizeTaskEvent, type TaskActivityView } from "../lib/task-activity-copy";
import { resolveEventTone, visualFor } from "../lib/timeline-visuals";
import type { TaskTimelineItem } from "../types";

/**
 * One humanized activity row: plain-language title first, optional detail,
 * freshness right-aligned, and the raw `event_type · seq` kept as quiet mono
 * microtext for operators.
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
      meta={
        <span className="font-mono text-micro text-faint">
          {item.event_type} · {item.sequence}
        </span>
      }
      time={item.timestamp ? <Time iso={item.timestamp} mode="relative" /> : undefined}
      title={view.title}
      tone={tone}
    />
  );
}
