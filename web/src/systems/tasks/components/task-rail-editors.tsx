import { ChevronsUpDown } from "lucide-react";

import {
  cn,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
  Switch,
} from "@agh/ui";

import { taskPriorityPresentation } from "../lib/task-properties-presentation";
import type { TaskPriority } from "../types";

const PRIORITY_ORDER: readonly TaskPriority[] = ["urgent", "high", "medium", "low"];

/** Inline PATCH-backed priority editor for the properties rail. */
export function TaskPriorityEditor({
  priority,
  pending,
  onChange,
}: {
  priority: TaskPriority;
  pending: boolean;
  onChange: (priority: TaskPriority) => void;
}) {
  const presentation = taskPriorityPresentation(priority);
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label="Change priority"
        data-testid="tasks-rail-priority"
        disabled={pending}
        render={
          <button
            className={cn(
              "-mr-1.5 inline-flex items-center gap-1.5 rounded-sm px-1.5 py-0.5",
              "text-small-body font-medium text-fg transition-colors duration-fast",
              "hover:bg-row-hover focus-visible:outline-none focus-visible:shadow-focus-ring",
              "disabled:cursor-not-allowed disabled:opacity-50"
            )}
            type="button"
          />
        }
      >
        <span aria-hidden="true" className={cn("size-1.5 rounded-full", presentation.dotClass)} />
        {presentation.label}
        <ChevronsUpDown aria-hidden="true" className="size-[11px] text-faint" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" data-testid="tasks-rail-priority-menu">
        <DropdownMenuRadioGroup
          onValueChange={value => onChange(value as TaskPriority)}
          value={priority}
        >
          {PRIORITY_ORDER.map(option => (
            <TaskPriorityOption key={option} priority={option} />
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function TaskPriorityOption({ priority }: { priority: TaskPriority }) {
  const presentation = taskPriorityPresentation(priority);
  return (
    <DropdownMenuRadioItem data-testid={`tasks-rail-priority-${priority}`} value={priority}>
      <span aria-hidden="true" className={cn("size-1.5 rounded-full", presentation.dotClass)} />
      {presentation.label}
    </DropdownMenuRadioItem>
  );
}

/** Inline PATCH-backed auto-enqueue toggle (instant-effect boolean → switch). */
export function TaskAutoEnqueueSwitch({
  enabled,
  pending,
  onChange,
}: {
  enabled: boolean;
  pending: boolean;
  onChange: (enabled: boolean) => void;
}) {
  return (
    <Switch
      aria-label="Auto-enqueue when ready"
      checked={enabled}
      data-testid="tasks-rail-auto-enqueue"
      disabled={pending}
      onCheckedChange={onChange}
    />
  );
}
