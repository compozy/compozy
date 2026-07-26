import type { Filter, FilterFieldsConfig } from "@agh/ui";

import type { LoopRunStatus } from "../types";
import {
  type LoopCatalogFilter,
  type LoopKindFilter,
  type LoopStatusFilter,
  loopKind,
} from "./loop-catalog";
import { isLoopRunStatus, loopStatusLabel } from "./loop-formatters";

export type LoopFilterFieldKey = "kind" | "category" | "status";

export interface LoopFilterHandlers {
  onKindChange: (next: LoopKindFilter) => void;
  onCategoryChange: (next: string | null) => void;
  onStatusChange: (next: LoopStatusFilter | null) => void;
}

const KIND_OPTIONS: { value: Exclude<LoopKindFilter, "all">; label: string }[] = [
  { value: "read-only", label: "Read-only" },
  { value: "workspace", label: "Workspace" },
];

export function buildLoopFilterFields(
  categories: readonly string[],
  statuses: readonly LoopStatusFilter[]
): FilterFieldsConfig<string> {
  return [
    {
      key: "kind",
      label: "Kind",
      type: "select",
      options: KIND_OPTIONS.map(option => ({
        value: option.value,
        label: option.label,
      })),
    },
    {
      key: "category",
      label: "Category",
      type: "select",
      options: categories.map(value => ({ value, label: value })),
    },
    {
      key: "status",
      label: "Status",
      type: "select",
      options: statuses.map(value => ({
        value,
        label: loopStatusLabel(value),
      })),
    },
  ];
}

export function loopFiltersToChips(state: LoopCatalogFilter): Filter<string>[] {
  const chips: Filter<string>[] = [];
  if (state.kind !== "all") {
    chips.push({
      id: "loop-filter-kind",
      field: "kind",
      operator: "is",
      values: [state.kind],
    });
  }
  if (state.category) {
    chips.push({
      id: "loop-filter-category",
      field: "category",
      operator: "is",
      values: [state.category],
    });
  }
  if (state.status) {
    chips.push({
      id: "loop-filter-status",
      field: "status",
      operator: "is",
      values: [state.status],
    });
  }
  return chips;
}

export function applyLoopFilterChips(chips: Filter<string>[], handlers: LoopFilterHandlers): void {
  const lookup = new Map<string, string | undefined>();
  for (const chip of chips) {
    lookup.set(chip.field, chip.values[0]);
  }
  handlers.onKindChange(asLoopKind(lookup.get("kind")) ?? "all");
  handlers.onCategoryChange(asNonEmptyString(lookup.get("category")));
  handlers.onStatusChange(asLoopStatus(lookup.get("status")));
}

export function parseLoopKindFilter(value: unknown): Exclude<LoopKindFilter, "all"> | undefined {
  if (typeof value !== "string") return undefined;
  return asLoopKind(value) ?? undefined;
}

export function parseLoopCategoryFilter(value: unknown): string | undefined {
  if (typeof value !== "string") return undefined;
  return asNonEmptyString(value) ?? undefined;
}

export function parseLoopStatusFilter(value: unknown): LoopRunStatus | undefined {
  if (typeof value !== "string") return undefined;
  return asLoopStatus(value) ?? undefined;
}

function asLoopKind(value: string | undefined): Exclude<LoopKindFilter, "all"> | null {
  if (value === "read-only" || value === "workspace") return value;
  return null;
}

function asNonEmptyString(value: string | undefined): string | null {
  if (!value) return null;
  const trimmed = value.trim();
  return trimmed === "" ? null : trimmed;
}

function asLoopStatus(value: string | undefined): LoopStatusFilter | null {
  if (!value) return null;
  return isLoopRunStatus(value) ? value : null;
}

/** Re-export for callers that need kind classification without importing loop-catalog. */
export { loopKind };
