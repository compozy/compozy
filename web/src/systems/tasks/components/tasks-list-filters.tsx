import { ListFilter } from "@compozy/ui/icons";

import { Button } from "@compozy/ui";
import { FiltersWithSearch, type Filter, type FilterFieldConfig } from "@compozy/ui";
import {
  projectFilterChip,
  projectFilterValuesFromChips,
  type ProjectFilterValue,
} from "@/systems/workspace";

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
  /** The quick pills carry the active status, so no duplicate status chip renders. */
  statusShownElsewhere?: boolean;
  onStatusChange: TaskFilterHandlers["onStatusChange"];
  onOwnerChange: TaskFilterHandlers["onOwnerChange"];
  onPriorityChange: TaskFilterHandlers["onPriorityChange"];
  /** The shared Project filter field; present only under All projects. */
  projectField?: FilterFieldConfig<string> | null;
  projectValues?: readonly ProjectFilterValue[];
  onProjectChange?: (next: ProjectFilterValue[]) => void;
}

export function TasksListFilters({
  statusFilter,
  ownerFilter,
  priorityFilter,
  ownerOptions,
  onStatusChange,
  onOwnerChange,
  onPriorityChange,
  statusShownElsewhere = false,
  projectField = null,
  projectValues = [],
  onProjectChange,
}: TasksListFiltersProps) {
  // Project leads: it decides which projects the other filters search.
  const fields = [...(projectField ? [projectField] : []), ...buildTaskFilterFields(ownerOptions)];
  const chipOptions = { statusShownElsewhere };
  const projectChip = projectField ? projectFilterChip(projectValues) : null;
  const chips = [
    ...(projectChip ? [projectChip] : []),
    ...taskFiltersToChips({ statusFilter, ownerFilter, priorityFilter }, chipOptions),
  ];

  const handleFiltersChange = (next: Filter<string>[]) => {
    applyTaskFilterChips(next, { onStatusChange, onOwnerChange, onPriorityChange }, chipOptions);
    if (projectField) onProjectChange?.(projectFilterValuesFromChips(next));
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
            size="segment"
            type="button"
            variant="quiet"
          >
            <ListFilter aria-hidden="true" />
            <span className="hidden @md/tasks-strip:inline">Filter</span>
          </Button>
        }
      />
    </div>
  );
}
