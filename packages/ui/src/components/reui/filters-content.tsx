import { cn } from "../../lib/utils";
import { ButtonGroup, ButtonGroupText } from "../button-group";
import { FilterOperatorDropdown, FilterRemoveButton } from "./filter-controls";
import { filtersContainerVariants } from "./filter-layout";
import type { Filter, FilterFieldsConfig } from "./filter-types";
import { FilterValueSelector } from "./filter-value-selector";
import { getFieldsMap } from "./hooks/filter-helpers";
import { useFilterContext } from "./hooks/use-filter-context";

interface FiltersContentProps<T = unknown> {
  filters: Filter<T>[];
  fields: FilterFieldsConfig<T>;
  onChange: (filters: Filter<T>[]) => void;
  focusFilterId?: string | null;
}

function FiltersContent<T = unknown>({
  filters,
  fields,
  onChange,
  focusFilterId = null,
}: FiltersContentProps<T>) {
  const context = useFilterContext();
  const fieldsMap = getFieldsMap(fields);

  const updateFilter = (filterId: string, updates: Partial<Filter<T>>) => {
    onChange(
      filters.map(filter => {
        if (filter.id !== filterId) return filter;
        const updatedFilter = { ...filter, ...updates };
        if (updates.operator === "empty" || updates.operator === "not_empty") {
          updatedFilter.values = [] as T[];
        }
        return updatedFilter;
      })
    );
  };

  const removeFilter = (filterId: string) => {
    onChange(filters.filter(filter => filter.id !== filterId));
  };

  return (
    <div
      className={cn(
        filtersContainerVariants({ variant: context.variant, size: context.size }),
        context.className
      )}
    >
      {filters.map(filter => {
        const field = fieldsMap[filter.field];
        if (!field) return null;

        if (field.type === "toggle") {
          return (
            <ButtonGroup key={filter.id}>
              <ButtonGroupText className="bg-background dark:bg-input/30">
                {field.icon}
                {field.label}
              </ButtonGroupText>
              <FilterRemoveButton onClick={() => removeFilter(filter.id)} />
            </ButtonGroup>
          );
        }

        return (
          <ButtonGroup key={filter.id}>
            <ButtonGroupText className="bg-background dark:bg-input/30">
              {field.icon}
              {field.label}
            </ButtonGroupText>
            <FilterOperatorDropdown<T>
              field={field}
              operator={filter.operator}
              values={filter.values}
              onChange={operator => updateFilter(filter.id, { operator })}
            />
            <FilterValueSelector<T>
              field={field}
              values={filter.values}
              onChange={values => updateFilter(filter.id, { values })}
              operator={filter.operator}
              focusOnMount={filter.id === focusFilterId}
            />
            <FilterRemoveButton onClick={() => removeFilter(filter.id)} />
          </ButtonGroup>
        );
      })}
    </div>
  );
}

export { FiltersContent };
