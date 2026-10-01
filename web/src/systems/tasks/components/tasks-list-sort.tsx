import { ArrowUpDown, ChevronDown } from "lucide-react";

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
            size="segment"
            type="button"
            variant="quiet"
          />
        }
      >
        <ArrowUpDown aria-hidden="true" className="text-subtle" />
        <span className="hidden @4xl/tasks-strip:inline">{SORT_LABELS[sortBy]}</span>
        <ChevronDown
          aria-hidden="true"
          className="hidden size-3 text-subtle @4xl/tasks-strip:inline"
        />
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
