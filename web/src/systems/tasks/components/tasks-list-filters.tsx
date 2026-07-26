import { ListFilter } from "lucide-react";

import { Button } from "@agh/ui";
import { FiltersWithSearch, type Filter } from "@agh/ui";

import {
  applyTaskFilterChips,
  buildTaskFilterFields,
  taskFiltersToChips,
  type TaskFilterHandlers,
  type TaskFilterOwnerOption,
  type TaskFilterState,
} from "../lib/tasks-list-filters";

export interface TasksListFiltersProps extends TaskFilterState {
  ownerOptions: TaskFilterOwnerOption[];
  onStatusChange: TaskFilterHandlers["onStatusChange"];
  onOwnerChange: TaskFilterHandlers["onOwnerChange"];
  onPriorityChange: TaskFilterHandlers["onPriorityChange"];
}

export function TasksListFilters({
  statusFilter,
  ownerFilter,
  priorityFilter,
  ownerOptions,
  onStatusChange,
  onOwnerChange,
  onPriorityChange,
}: TasksListFiltersProps) {
  const fields = buildTaskFilterFields(ownerOptions);
  const chips = taskFiltersToChips({ statusFilter, ownerFilter, priorityFilter });

  const handleFiltersChange = (next: Filter<string>[]) => {
    applyTaskFilterChips(next, {
      onStatusChange,
      onOwnerChange,
      onPriorityChange,
    });
  };

  return (
    <div className="flex flex-nowrap items-center gap-2" data-testid="tasks-list-filters">
      <FiltersWithSearch<string>
        allowMultiple={false}
        fields={fields}
        filters={chips}
        onChange={handleFiltersChange}
        size="sm"
        trigger={
          <Button
            aria-label="Add filter"
            data-testid="tasks-list-filters-add"
            size="sm"
            type="button"
            variant="ghost"
          >
            <ListFilter aria-hidden="true" className="size-3" />
            Filter
          </Button>
        }
      />
    </div>
  );
}
