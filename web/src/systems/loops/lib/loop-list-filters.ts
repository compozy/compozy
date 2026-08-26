import type { Filter, FilterFieldsConfig } from "@compozy/ui";

import { LOOP_RUN_STATUSES } from "@/generated/loop-enums";

import type { LoopRunStatus } from "../types";
import type { LoopStatusFilter } from "./loop-catalog";
import { isLoopRunStatus, loopStatusLabel } from "./loop-formatters";
import type { LoopOutcomeValue } from "./loop-runs-view";

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

export type LoopRunOriginFilter = "catalog" | "session";

export interface LoopRunFilterState {
  origin?: LoopRunOriginFilter;
  originSession?: string;
  outcome: LoopOutcomeValue;
}

export interface LoopRunFilterHandlers {
  onOriginFilterChange: (next: { origin?: LoopRunOriginFilter; originSession?: string }) => void;
  onOutcomeChange: (next: LoopOutcomeValue) => void;
}

function isLoopRunOriginFilter(value: unknown): value is LoopRunOriginFilter {
  return value === "catalog" || value === "session";
}

/**
 * Runs-roster chip-bar fields: origin (server query), session id (server query),
 * and outcome (client partition filter). The outcome select carries the full
 * generated status vocabulary and no counts — filtering for a status the loaded
 * window lacks must reach the truthful empty state, never a hidden option (SD-007).
 */
export function buildLoopRunFilterFields(): FilterFieldsConfig<string> {
  return [
    {
      key: "origin",
      label: "Origin",
      type: "select",
      options: [
        { value: "catalog", label: "Catalog" },
        { value: "session", label: "Session" },
      ],
    },
    { key: "origin_session", label: "Session id", type: "text" },
    { key: "outcome", label: "Outcome", type: "select", options: loopStatusFilterOptions() },
  ];
}

/**
 * Projects committed filter state into chips. A session id implies the session
 * origin, so it renders as the single session-id chip rather than a chip pair.
 */
export function loopRunFiltersToChips(state: LoopRunFilterState): Filter<string>[] {
  const chips: Filter<string>[] = [];
  if (state.originSession !== undefined && state.originSession !== "") {
    chips.push({
      id: "loop-run-filter-session",
      field: "origin_session",
      operator: "is",
      values: [state.originSession],
    });
  } else if (state.origin) {
    chips.push({
      id: "loop-run-filter-origin",
      field: "origin",
      operator: "is",
      values: [state.origin],
    });
  }
  if (state.outcome !== "all") {
    chips.push({
      id: "loop-run-filter-outcome",
      field: "outcome",
      operator: "is",
      values: [state.outcome],
    });
  }
  return chips;
}

/**
 * Resolves chips into filter state and dispatches it. A session-id chip forces
 * `origin=session` even while its text is empty (the pre-chip input behaved the
 * same way: clearing the id kept the origin); removing the chip clears both.
 */
export function applyLoopRunFilterChips(
  chips: Filter<string>[],
  handlers: LoopRunFilterHandlers
): LoopRunFilterState {
  const sessionChip = chips.find(chip => chip.field === "origin_session");
  const originValue = chips.find(chip => chip.field === "origin")?.values[0];
  const outcomeValue = chips.find(chip => chip.field === "outcome")?.values[0];
  const sessionText = typeof sessionChip?.values[0] === "string" ? sessionChip.values[0] : "";
  const outcome: LoopOutcomeValue = isLoopRunStatus(outcomeValue) ? outcomeValue : "all";
  const resolved: LoopRunFilterState = sessionChip
    ? {
        origin: "session",
        originSession: sessionText === "" ? undefined : sessionText,
        outcome,
      }
    : { origin: isLoopRunOriginFilter(originValue) ? originValue : undefined, outcome };
  handlers.onOriginFilterChange({ origin: resolved.origin, originSession: resolved.originSession });
  handlers.onOutcomeChange(resolved.outcome);
  return resolved;
}

export {
  parseLoopCategoryFilter,
  parseLoopKindFilter,
  parseLoopStatusFilter,
} from "./loops-route-search";
