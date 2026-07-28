import { ChevronDown, ListFilter } from "lucide-react";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@compozy/ui";

import type { TaskListSortKey } from "../types";
import { TASK_LIST_SORT_OPTIONS } from "../lib/tasks-list-filters";

const SORT_LABELS: Record<TaskListSortKey, string> = {
  recent: "Most recent",
  priority: "Priority",
};

export interface TasksListSortProps {
  sortBy: TaskListSortKey;
  onSortChange: (next: TaskListSortKey) => void;
}

export function TasksListSort({ sortBy, onSortChange }: TasksListSortProps) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            aria-label="Sort tasks"
            data-testid="tasks-list-sort-trigger"
            size="sm"
            type="button"
            variant="ghost"
          />
        }
      >
        <ListFilter aria-hidden="true" className="size-3 text-subtle" />
        <span className="text-muted">Sorted by</span>
        <span className="text-fg-strong">{SORT_LABELS[sortBy]}</span>
        <ChevronDown aria-hidden="true" className="size-3 text-subtle" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        {TASK_LIST_SORT_OPTIONS.map(option => (
          <DropdownMenuItem
            data-active={option === sortBy ? "true" : undefined}
            data-testid={`tasks-list-sort-${option}`}
            key={option}
            onSelect={event => {
              event.preventDefault();
              onSortChange(option);
            }}
          >
            {SORT_LABELS[option]}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
