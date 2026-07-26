import type { Filter, FilterFieldsConfig } from "@agh/ui";

import type { AgentFleetStatus } from "./fleet-signals";
import type { AgentsFleetSearch } from "./agent-fleet-search";

export type AgentFleetFilterFieldKey = "category" | "status";

export interface AgentFleetFilterValues {
  category?: string;
  status?: AgentFleetStatus;
}

const STATUS_OPTIONS: { value: AgentFleetStatus; label: string }[] = [
  { value: "active", label: "Active" },
  { value: "idle", label: "Idle" },
];

export function buildAgentFleetFilterFields(
  categories: readonly string[]
): FilterFieldsConfig<string> {
  return [
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
      options: STATUS_OPTIONS.map(option => ({
        value: option.value,
        label: option.label,
      })),
    },
  ];
}

export function agentFleetFiltersToChips(search: AgentsFleetSearch): Filter<string>[] {
  const chips: Filter<string>[] = [];
  if (search.category) {
    chips.push({
      id: "agent-fleet-filter-category",
      field: "category",
      operator: "is",
      values: [search.category],
    });
  }
  if (search.status) {
    chips.push({
      id: "agent-fleet-filter-status",
      field: "status",
      operator: "is",
      values: [search.status],
    });
  }
  return chips;
}

function asStatus(value: string | undefined): AgentFleetStatus | undefined {
  return value === "active" || value === "idle" ? value : undefined;
}

function asNonEmptyString(value: string | undefined): string | undefined {
  if (!value) return undefined;
  const trimmed = value.trim();
  return trimmed === "" ? undefined : trimmed;
}

export function agentFleetChipsToFilters(chips: Filter<string>[]): AgentFleetFilterValues {
  const lookup = new Map<string, string | undefined>();
  for (const chip of chips) {
    lookup.set(chip.field, chip.values[0]);
  }
  return {
    category: asNonEmptyString(lookup.get("category")),
    status: asStatus(lookup.get("status")),
  };
}
