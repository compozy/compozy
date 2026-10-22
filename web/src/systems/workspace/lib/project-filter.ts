import type { Filter, FilterFieldConfig } from "@compozy/ui";

import { USER_SCOPE, type ProjectFilterValue } from "./breadth-query";
import { DESTINATION_COPY } from "./workspace-scope-copy";

/** The listing-toolbar field key of the shared Project filter. */
export const PROJECT_FILTER_FIELD = "project";

const PROJECT_FILTER_OPERATOR = "is_any_of";

export const PROJECT_FILTER_COPY = {
  label: "Project",
  operator: "is any of",
} as const;

/**
 * The shared Project filter field (multi-select, searchable): the projects in
 * switcher order, then **User**, always last. Only the inclusive operator is
 * offered, because the daemon narrows to the selected scopes and nothing else.
 */
export function projectFilterField(
  projects: ReadonlyArray<{ id: string; name: string }>
): FilterFieldConfig<string> {
  return {
    key: PROJECT_FILTER_FIELD,
    label: PROJECT_FILTER_COPY.label,
    type: "multiselect",
    searchable: true,
    operators: [{ value: PROJECT_FILTER_OPERATOR, label: PROJECT_FILTER_COPY.operator }],
    defaultOperator: PROJECT_FILTER_OPERATOR,
    options: [
      ...projects.map(project => ({ value: project.id, label: project.name })),
      { value: USER_SCOPE, label: DESTINATION_COPY.user },
    ],
  };
}

/** The chip a non-empty Project filter renders as; empty means every project, so no chip. */
export function projectFilterChip(values: readonly ProjectFilterValue[]): Filter<string> | null {
  if (values.length === 0) return null;
  return {
    id: `listing-filter-${PROJECT_FILTER_FIELD}`,
    field: PROJECT_FILTER_FIELD,
    operator: PROJECT_FILTER_OPERATOR,
    values: [...values],
  };
}

/** The Project filter a chip array carries; a removed chip is the empty (all) filter. */
export function projectFilterValuesFromChips(
  chips: readonly Filter<string>[]
): ProjectFilterValue[] {
  return chips.find(chip => chip.field === PROJECT_FILTER_FIELD)?.values ?? [];
}
