import { PillGroup, type PillGroupItem } from "@compozy/ui";

import { taskPriorityPresentation } from "../../lib/task-properties-presentation";
import type { TaskPriority } from "../../types";

interface PrioritySectionProps {
  priority: TaskPriority;
  onPriority: (priority: TaskPriority) => void;
}

const PRIORITY_ORDER: readonly TaskPriority[] = ["low", "medium", "high", "urgent"];

const PRIORITY_ITEMS: PillGroupItem<TaskPriority>[] = PRIORITY_ORDER.map(value => {
  const { dotClass, label } = taskPriorityPresentation(value);
  return {
    value,
    label: (
      <span className="flex items-center gap-1.5">
        <span aria-hidden="true" className={`size-1.5 rounded-full ${dotClass}`} />
        {label}
      </span>
    ),
    testId: `task-priority-${value}`,
  };
});

/**
 * Task priority selector — a segmented PillGroup whose segments carry the
 * shared neutral priority dot ahead of each label. Higher priority is claimed sooner.
 */
export function PrioritySection({ priority, onPriority }: PrioritySectionProps) {
  return (
    <PillGroup
      aria-label="Task priority"
      data-testid="task-priority-value"
      items={PRIORITY_ITEMS}
      onChange={onPriority}
      size="sm"
      value={priority}
    />
  );
}
