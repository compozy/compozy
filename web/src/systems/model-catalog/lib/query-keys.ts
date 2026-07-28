export const modelCatalogKeys = {
  all: ["model-catalog"] as const,
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
