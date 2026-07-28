import type {
  Filter,
  FilterFieldConfig,
  FilterFieldGroup,
  FilterFieldsConfig,
} from "../filter-types";

export const isFieldGroup = <T = unknown>(
  item: FilterFieldConfig<T> | FilterFieldGroup<T>
): item is FilterFieldGroup<T> => {
  return "fields" in item && Array.isArray(item.fields);
};

export const isGroupLevelField = <T = unknown>(field: FilterFieldConfig<T>): boolean => {
  return Boolean(field.group && field.fields);
};

export const flattenFields = <T = unknown>(
  fields: FilterFieldsConfig<T>
): FilterFieldConfig<T>[] => {
  const flattened: FilterFieldConfig<T>[] = [];
  for (const item of fields) {
    if (isFieldGroup(item)) {
      flattened.push(...item.fields);
    } else if (isGroupLevelField(item) && item.fields) {
      flattened.push(...item.fields);
    } else {
      flattened.push(item);
    }
  }
  return flattened;
};

export const getFieldsMap = <T = unknown>(
  fields: FilterFieldsConfig<T>
): Record<string, FilterFieldConfig<T>> => {
  const flatFields = flattenFields(fields);
  return flatFields.reduce(
    (acc, field) => {
      if (field.key) {
        acc[field.key] = field;
      }
      return acc;
    },
    {} as Record<string, FilterFieldConfig<T>>
  );
};

export const createFilter = <T = unknown>(
  field: string,
  operator?: string,
  values: T[] = []
): Filter<T> => ({
  id: `${Date.now()}-${Math.random().toString(36).substring(2, 11)}`,
  field,
  operator: operator || "is",
  values,
});
