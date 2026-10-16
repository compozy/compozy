import { useQuery } from "@tanstack/react-query";

import { allModelsListOptions } from "@/systems/model-catalog";
import { providerListOptions } from "@/systems/providers";

import type { SessionRuntimeEffective } from "../types";

export interface SessionRuntimeLabel {
  /** Registry key for the provider mark. */
  provider: string;
  /** `Provider · Model` in the names the runtime selector shows. */
  label: string;
}

/**
 * Names a session's effective runtime the way the runtime selector does: the
 * provider and model display names from the same shared catalog reads, and the
 * wire id only where the catalog has no name. Every row shares those two cached
 * reads, so a page of rows adds no request per row. `null` before the first bind.
 */
export function useSessionRuntimeLabel(
  effective: SessionRuntimeEffective | null | undefined
): SessionRuntimeLabel | null {
  const provider = effective?.provider.trim() ?? "";
  const model = effective?.model?.trim() ?? "";
  const providerName = useQuery({
    ...providerListOptions(),
    enabled: provider !== "",
    select: data =>
      data.providers.find(entry => entry.name === provider)?.display_name?.trim() ?? "",
  }).data;
  const modelName = useQuery({
    ...allModelsListOptions({
      view: "all",
      includeStale: true,
      enabled: provider !== "" && model !== "",
    }),
    select: data =>
      data.models
        .find(entry => entry.provider_id === provider && entry.model_id === model)
        ?.display_name?.trim() ?? "",
  }).data;
  if (provider === "") return null;
  const providerLabel = providerName || provider;
  const modelLabel = modelName || model;
  return { provider, label: modelLabel ? `${providerLabel} · ${modelLabel}` : providerLabel };
}
