import { PillGroup, type PillGroupItem } from "@agh/ui";

import type { TaskPriority } from "../../types";

interface PrioritySectionProps {
  priority: TaskPriority;
  onPriority: (priority: TaskPriority) => void;
}

interface PriorityOption {
  value: TaskPriority;
  label: string;
  dot: string;
}

const PRIORITY_OPTIONS: PriorityOption[] = [
  { value: "low", label: "Low", dot: "bg-neutral" },
  { value: "medium", label: "Medium", dot: "bg-info" },
  { value: "high", label: "High", dot: "bg-warning" },
  { value: "urgent", label: "Urgent", dot: "bg-accent-strong" },
];

const PRIORITY_ITEMS: PillGroupItem<TaskPriority>[] = PRIORITY_OPTIONS.map(option => ({
  value: option.value,
  label: (
    <span className="flex items-center gap-1.5">
      <span aria-hidden="true" className={`size-1.5 rounded-full ${option.dot}`} />
      {option.label}
    </span>
  ),
  testId: `task-priority-${option.value}`,
}));

/**
 * Task priority selector — a segmented PillGroup whose segments carry a
 * signal-colored dot ahead of each label. Higher priority is claimed sooner.
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
