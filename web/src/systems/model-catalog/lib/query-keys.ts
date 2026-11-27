import { computerQueryKey, LOCAL_COMPUTER_ID } from "@/lib/computer-scope";

export const modelCatalogKeys = {
  all: computerQueryKey(LOCAL_COMPUTER_ID, "model-catalog"),
  /** One browser-lifetime readiness refresh, separate from catalog invalidation. */
  initialReadiness: computerQueryKey(LOCAL_COMPUTER_ID, "model-catalog-readiness", "initial"),
  /** Aggregate cross-provider list (`GET /api/model-catalog/models`). */
  allModels: (
    view?: "curated" | "all",
    providerId?: string,
    sourceId?: string,
    includeStale?: boolean
  ) =>
    [
      ...modelCatalogKeys.all,
      "models",
      view ?? "curated",
      providerId ?? "",
      sourceId ?? "",
      Boolean(includeStale),
    ] as const,
  sourceStatus: computerQueryKey(LOCAL_COMPUTER_ID, "model-catalog", "sources", "status"),
  providers: () => [...modelCatalogKeys.all, "providers"] as const,
  providerRoot: (providerId: string) =>
    [...modelCatalogKeys.providers(), providerId.trim()] as const,
  providerModels: (
    providerId: string,
    sourceId?: string,
    includeStale?: boolean,
    view?: "curated" | "all"
  ) =>
    [
      ...modelCatalogKeys.providerRoot(providerId),
      "models",
      sourceId ?? "",
      Boolean(includeStale),
      view ?? "curated",
    ] as const,
  providerStatus: (providerId: string) =>
    [...modelCatalogKeys.providerRoot(providerId), "status"] as const,
};
