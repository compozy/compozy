import type { Filter, FilterFieldsConfig } from "@compozy/ui";

import { LOOP_RUN_STATUSES } from "@/generated/loop-enums";

import type { LoopRunStatus } from "../types";
import { type LoopKindFilter, type LoopStatusFilter, loopKind } from "./loop-catalog";
import { isLoopRunStatus, loopStatusLabel } from "./loop-formatters";

export interface LoopStatusFilterOption {
  value: LoopRunStatus;
  label: string;
}

export interface LoopFilterState {
  status: LoopStatusFilter | null;
}

export interface LoopFilterHandlers {
  onStatusChange: (next: LoopStatusFilter | null) => void;
}

/**
 * Every status the daemon can report for a Loop's latest run, in contract order.
 *
 * Sourced from the generated enum rather than from the catalog facets: a status the
 * current page happens not to contain is still a legal `?status=` value, so filtering
 * for `canceled` on a roster with no canceled run must reach the truthful empty state
 * instead of the option silently disappearing.
 */
export function loopStatusFilterOptions(): LoopStatusFilterOption[] {
  return LOOP_RUN_STATUSES.map(value => ({ value, label: loopStatusLabel(value) }));
}

/** Chip-bar field config: one select over the daemon's full latest-run status vocabulary. */
export function buildLoopFilterFields(): FilterFieldsConfig<string> {
  return [{ key: "status", label: "Status", type: "select", options: loopStatusFilterOptions() }];
}

export function loopFiltersToChips(state: LoopFilterState): Filter<string>[] {
  if (!state.status) return [];
  return [{ id: "loop-filter-status", field: "status", operator: "is", values: [state.status] }];
}

export function applyLoopFilterChips(chips: Filter<string>[], handlers: LoopFilterHandlers): void {
  const status = chips.find(chip => chip.field === "status")?.values[0];
  handlers.onStatusChange(isLoopRunStatus(status) ? status : null);
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
  return isLoopRunStatus(value) ? value : undefined;
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

/** Re-export for callers that need kind classification without importing loop-catalog. */
export { loopKind };
