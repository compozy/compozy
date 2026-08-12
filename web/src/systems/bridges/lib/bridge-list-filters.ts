import type { Filter, FilterFieldsConfig } from "@compozy/ui";

import { bridgeStatusLabel } from "./bridge-formatters";
import {
  bridgePlatformFilterFrom,
  bridgeScopeFilterFrom,
  bridgeStatusFilterFrom,
  type BridgePlatformFilter,
  type BridgeStatusFilter,
} from "./bridge-route-search";
import type {
  BridgeHealthMap,
  BridgeListFilter,
  BridgeScopeFilter,
  BridgeStatus,
  BridgeSummary,
} from "../types";

export type BridgeFilterFieldKey = "scope" | "platform" | "status";

export interface BridgeFilterState {
  scope: BridgeScopeFilter;
  platform: BridgePlatformFilter | null;
  status: BridgeStatusFilter | null;
}

export interface BridgeFilterHandlers {
  onScopeChange: (next: BridgeScopeFilter) => void;
  onPlatformChange: (next: BridgePlatformFilter | null) => void;
  onStatusChange: (next: BridgeStatusFilter | null) => void;
}

export function bridgeListFilterForScope(
  scope: BridgeScopeFilter,
  activeWorkspaceId: string | null
): BridgeListFilter {
  if (scope === "global" || (scope === "all" && !activeWorkspaceId)) {
    return { scope: "global" };
  }
  if (!activeWorkspaceId) {
    return { scope: "workspace" };
  }
  return { scope: scope === "all" ? "all" : scope, workspace_id: activeWorkspaceId };
}

const SCOPE_OPTIONS: { value: BridgeScopeFilter; label: string }[] = [
  { value: "all", label: "All" },
  { value: "global", label: "Global" },
  { value: "workspace", label: "Workspace" },
];

const STATUS_OPTIONS: readonly BridgeStatusFilter[] = [
  "disabled",
  "starting",
  "ready",
  "degraded",
  "auth_required",
  "error",
];

export function buildBridgeFilterFields(
  platforms: readonly string[],
  statuses: readonly BridgeStatusFilter[] = STATUS_OPTIONS
): FilterFieldsConfig<string> {
  const distinctPlatforms = [...new Set(platforms)].sort((left, right) =>
    left.localeCompare(right)
  );

  return [
    {
      key: "scope",
      label: "Scope",
      type: "select",
      options: SCOPE_OPTIONS,
    },
    {
      key: "platform",
      label: "Platform",
      type: "select",
      options: distinctPlatforms.map(value => ({ value, label: value })),
    },
    {
      key: "status",
      label: "Status",
      type: "select",
      options: statuses.map(value => ({
        value,
        label: bridgeStatusLabel(value),
      })),
    },
  ];
}

export function bridgeFiltersToChips(state: BridgeFilterState): Filter<string>[] {
  const chips: Filter<string>[] = [];
  if (state.scope !== "all") {
    chips.push({
      id: "bridge-filter-scope",
      field: "scope",
      operator: "is",
      values: [state.scope],
    });
  }
  if (state.platform) {
    chips.push({
      id: "bridge-filter-platform",
      field: "platform",
      operator: "is",
      values: [state.platform],
    });
  }
  if (state.status) {
    chips.push({
      id: "bridge-filter-status",
      field: "status",
      operator: "is",
      values: [state.status],
    });
  }
  return chips;
}

export function applyBridgeFilterChips(
  chips: Filter<string>[],
  handlers: BridgeFilterHandlers
): void {
  const lookup = new Map<string, string | undefined>();
  for (const chip of chips) {
    lookup.set(chip.field, chip.values[0]);
  }
  handlers.onScopeChange(bridgeScopeFilterFrom(lookup.get("scope")) ?? "all");
  handlers.onPlatformChange(bridgePlatformFilterFrom(lookup.get("platform")));
  handlers.onStatusChange(bridgeStatusFilterFrom(lookup.get("status")));
}

export function effectiveBridgeStatus(
  bridge: BridgeSummary,
  health: BridgeHealthMap[string] | undefined
): BridgeStatus {
  return health?.status ?? bridge.status;
}

export type { BridgePlatformFilter, BridgeStatusFilter } from "./bridge-route-search";
