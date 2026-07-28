import { normalizeBridgeCatalogFilter } from "./bridge-list-query";
import type { BridgeCatalogFilter, BridgeTargetsQuery } from "../types";

function normalizeKeyValue(value?: string | number | null) {
  return value == null ? "" : String(value);
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
    ] as const;
  },
  providers: () => [...bridgeKeys.all, "providers"] as const,
  manifestsRoot: () => [...bridgeKeys.all, "manifest"] as const,
  slackManifest: (instanceID: string) =>
    [...bridgeKeys.manifestsRoot(), "slack", instanceID.trim()] as const,
  details: () => [...bridgeKeys.all, "detail"] as const,
  detail: (id: string) => [...bridgeKeys.details(), normalizeKeyValue(id)] as const,
  routesRoot: () => [...bridgeKeys.all, "routes"] as const,
  routes: (id: string) => [...bridgeKeys.routesRoot(), normalizeKeyValue(id)] as const,
  targetsRoot: () => [...bridgeKeys.all, "targets"] as const,
  targetsForBridge: (id: string) => [...bridgeKeys.targetsRoot(), normalizeKeyValue(id)] as const,
  targets: (id: string, query: BridgeTargetsQuery = {}) =>
    [
      ...bridgeKeys.targetsForBridge(id),
      normalizeKeyValue(query.q),
      normalizeKeyValue(query.limit),
    ] as const,
  secretBindingsRoot: () => [...bridgeKeys.all, "secret-bindings"] as const,
  secretBindings: (id: string) =>
    [...bridgeKeys.secretBindingsRoot(), normalizeKeyValue(id)] as const,
};
