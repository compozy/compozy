import type { Filter, FilterFieldsConfig } from "@agh/ui";

import { bridgeStatusLabel } from "./bridge-formatters";
import type {
  BridgeHealthMap,
  BridgeListFilter,
  BridgeScopeFilter,
  BridgeStatus,
  BridgeSummary,
} from "../types";

export type BridgePlatformFilter = string;
export type BridgeStatusFilter = BridgeStatus;

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

const STATUS_OPTIONS: BridgeStatusFilter[] = [
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
  handlers.onScopeChange(asBridgeScope(lookup.get("scope")) ?? "all");
  handlers.onPlatformChange(asBridgePlatform(lookup.get("platform")));
  handlers.onStatusChange(asBridgeStatus(lookup.get("status")));
}

export function effectiveBridgeStatus(
  bridge: BridgeSummary,
  health: BridgeHealthMap[string] | undefined
): BridgeStatus {
  return health?.status ?? bridge.status;
}

export function parseBridgeScopeFilter(value: unknown): BridgeScopeFilter | undefined {
  if (typeof value !== "string") return undefined;
  const scope = asBridgeScope(value);
  return scope === "all" ? undefined : (scope ?? undefined);
}

export function parseBridgePlatformFilter(value: unknown): BridgePlatformFilter | undefined {
  if (typeof value !== "string") return undefined;
  const trimmed = value.trim();
  return trimmed === "" ? undefined : trimmed;
}

export function parseBridgeStatusFilter(value: unknown): BridgeStatusFilter | undefined {
  if (typeof value !== "string") return undefined;
  return asBridgeStatus(value) ?? undefined;
}

function asBridgeScope(value: string | undefined): BridgeScopeFilter | null {
  if (!value) return null;
  if (value === "all" || value === "global" || value === "workspace") return value;
  return null;
}

function asBridgePlatform(value: string | undefined): BridgePlatformFilter | null {
  if (!value) return null;
  const trimmed = value.trim();
  return trimmed === "" ? null : trimmed;
}

function asBridgeStatus(value: string | undefined): BridgeStatusFilter | null {
  if (!value) return null;
  return (STATUS_OPTIONS as readonly string[]).includes(value)
    ? (value as BridgeStatusFilter)
    : null;
}
