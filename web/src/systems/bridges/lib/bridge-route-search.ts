import type { ListingViewMode } from "@compozy/ui";

import { normalizeListingSearchValue, parseListingView } from "@/lib/listing-search";
import type { BridgeScopeFilter, BridgeStatus } from "../types";

export type BridgePlatformFilter = string;
export type BridgeStatusFilter = BridgeStatus;

export interface BridgesRouteSearch {
  q?: string;
  view?: ListingViewMode;
  scope?: BridgeScopeFilter;
  platform?: BridgePlatformFilter;
  status?: BridgeStatusFilter;
}

export const BRIDGE_STATUSES: readonly BridgeStatusFilter[] = [
  "disabled",
  "starting",
  "ready",
  "degraded",
  "auth_required",
  "error",
];

export function bridgeScopeFilterFrom(value: unknown): BridgeScopeFilter | null {
  return value === "all" || value === "global" || value === "workspace" ? value : null;
}

export function bridgePlatformFilterFrom(value: unknown): BridgePlatformFilter | null {
  if (typeof value !== "string") return null;
  const trimmed = value.trim();
  return trimmed === "" ? null : trimmed;
}

export function bridgeStatusFilterFrom(value: unknown): BridgeStatusFilter | null {
  return typeof value === "string" && (BRIDGE_STATUSES as readonly string[]).includes(value)
    ? (value as BridgeStatusFilter)
    : null;
}

export function parseBridgeScopeFilter(value: unknown): BridgeScopeFilter | undefined {
  const scope = bridgeScopeFilterFrom(value);
  return scope === "all" ? undefined : (scope ?? undefined);
}

export function parseBridgePlatformFilter(value: unknown): BridgePlatformFilter | undefined {
  return bridgePlatformFilterFrom(value) ?? undefined;
}

export function parseBridgeStatusFilter(value: unknown): BridgeStatusFilter | undefined {
  return bridgeStatusFilterFrom(value) ?? undefined;
}

export function validateBridgesSearch(search: Record<string, unknown>): BridgesRouteSearch {
  return {
    platform: parseBridgePlatformFilter(search.platform),
    q: normalizeListingSearchValue(search.q),
    scope: parseBridgeScopeFilter(search.scope),
    status: parseBridgeStatusFilter(search.status),
    view: parseListingView(search.view),
  };
}
