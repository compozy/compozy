import type { ChangeEvent, ReactNode } from "react";

import type { FilterI18nConfig } from "./hooks/use-filter-context";

export interface FilterOption<T = unknown> {
  value: T;
  label: string;
  icon?: ReactNode;
  metadata?: Record<string, unknown>;
  className?: string;
}

export interface FilterOperator {
  value: string;
  label: string;
  supportsMultiple?: boolean;
}

export interface CustomRendererProps<T = unknown> {
  field: FilterFieldConfig<T>;
  values: T[];
  onChange: (values: T[]) => void;
  operator: string;
}

export interface FilterFieldGroup<T = unknown> {
  group?: string;
  fields: FilterFieldConfig<T>[];
}

export type FilterFieldsConfig<T = unknown> = FilterFieldConfig<T>[] | FilterFieldGroup<T>[];

export interface FilterFieldConfig<T = unknown> {
  key?: string;
  label?: string;
  icon?: ReactNode;
  type?: "select" | "multiselect" | "text" | "custom" | "separator" | "toggle";
  group?: string;
  fields?: FilterFieldConfig<T>[];
  options?: FilterOption<T>[];
  operators?: FilterOperator[];
  customRenderer?: (props: CustomRendererProps<T>) => ReactNode;
  customValueRenderer?: (values: T[], options: FilterOption<T>[]) => ReactNode;
  placeholder?: string;
  searchable?: boolean;
  maxSelections?: number;
  min?: number;
  max?: number;
  step?: number;
  prefix?: string | ReactNode;
  suffix?: string | ReactNode;
  pattern?: string;
  validation?: (value: unknown) => boolean | { valid: boolean; message?: string };
  allowCustomValues?: boolean;
  className?: string;
  menuPopupClassName?: string;
  groupLabel?: string;
  onLabel?: string;
  offLabel?: string;
  onInputChange?: (event: ChangeEvent<HTMLInputElement>) => void;
  defaultOperator?: string;
  value?: T[];
  onValueChange?: (values: T[]) => void;
}

export interface Filter<T = unknown> {
  id: string;
  field: string;
  operator: string;
  values: T[];
}

export interface FilterGroup<T = unknown> {
  id: string;
  label?: string;
  filters: Filter<T>[];
  fields: FilterFieldConfig<T>[];
}

export interface FiltersProps<T = unknown> {
  filters: Filter<T>[];
  fields: FilterFieldsConfig<T>;
  onChange: (filters: Filter<T>[]) => void;
  className?: string;
  variant?: "solid" | "default";
  size?: "sm" | "default" | "lg";
  radius?: "default" | "full";
  i18n?: Partial<FilterI18nConfig>;
  trigger?: ReactNode;
  allowMultiple?: boolean;
  menuPopupClassName?: string;
  enableShortcut?: boolean;
  shortcutKey?: string;
  shortcutLabel?: string;
}
