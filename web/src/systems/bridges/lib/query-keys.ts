import { PROFILE_AGGREGATE, type ProfileScopeParams } from "@/systems/profiles";

import { normalizeBridgeCatalogFilter } from "./bridge-list-query";
import type { BridgeCatalogFilter, BridgeTargetsQuery } from "../types";

function normalizeKeyValue(value?: string | number | null) {
  return value == null ? "" : String(value);
}

function profileKeySegment(scope: ProfileScopeParams | BridgeCatalogFilter): string {
  return "all_profiles" in scope && scope.all_profiles === true
    ? PROFILE_AGGREGATE
    : normalizeKeyValue("profile" in scope ? scope.profile : undefined);
}

export const bridgeKeys = {
  all: ["bridges"] as const,
  lists: () => [...bridgeKeys.all, "list"] as const,
  list: (filters: BridgeCatalogFilter = {}) => {
    const normalized = normalizeBridgeCatalogFilter(filters);
    return [
      ...bridgeKeys.lists(),
      normalized.scope ?? "all",
      normalizeKeyValue(normalized.workspace_id),
      normalizeKeyValue(normalized.workspace),
      normalizeKeyValue(normalized.q),
      normalizeKeyValue(normalized.platform),
      normalizeKeyValue(normalized.status),
      normalizeKeyValue(normalized.sort),
      normalizeKeyValue(normalized.limit),
      // One profile's bridges and the labeled aggregate are two answers, so a
      // switch reads a different entry instead of inheriting the last one.
      profileKeySegment(normalized),
    ] as const;
  },
  providers: () => [...bridgeKeys.all, "providers"] as const,
  manifestsRoot: () => [...bridgeKeys.all, "manifest"] as const,
  slackManifest: (instanceID: string, scope: ProfileScopeParams) =>
    [...bridgeKeys.manifestsRoot(), "slack", instanceID, profileKeySegment(scope)] as const,
  details: () => [...bridgeKeys.all, "detail"] as const,
  /** Every lens's read of one bridge — the prefix a mutation invalidates. */
  detailFor: (id: string) => [...bridgeKeys.details(), normalizeKeyValue(id)] as const,
  // The same id answers differently per lens: found under its owner or the
  // aggregate, absent under a profile that does not own it.
  detail: (id: string, scope: ProfileScopeParams) =>
    [...bridgeKeys.detailFor(id), profileKeySegment(scope)] as const,
  routesRoot: () => [...bridgeKeys.all, "routes"] as const,
  routesFor: (id: string) => [...bridgeKeys.routesRoot(), normalizeKeyValue(id)] as const,
  routes: (id: string, scope: ProfileScopeParams) =>
    [...bridgeKeys.routesFor(id), profileKeySegment(scope)] as const,
  targetsRoot: () => [...bridgeKeys.all, "targets"] as const,
  targetsForBridge: (id: string) => [...bridgeKeys.targetsRoot(), normalizeKeyValue(id)] as const,
  targets: (id: string, query: BridgeTargetsQuery = {}) =>
    [
      ...bridgeKeys.targetsForBridge(id),
      normalizeKeyValue(query.q),
      normalizeKeyValue(query.limit),
      profileKeySegment(query),
    ] as const,
  secretBindingsRoot: () => [...bridgeKeys.all, "secret-bindings"] as const,
  secretBindingsFor: (id: string) =>
    [...bridgeKeys.secretBindingsRoot(), normalizeKeyValue(id)] as const,
  secretBindings: (id: string, scope: ProfileScopeParams) =>
    [...bridgeKeys.secretBindingsFor(id), profileKeySegment(scope)] as const,
};
