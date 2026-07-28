import { useQuery } from "@tanstack/react-query";

import type { RuntimeModelOption } from "@/systems/runtime";

import { allModelsListOptions } from "../lib/query-options";
import { toRuntimeModelOptions } from "../lib/to-runtime-selector-options";
import type { ProviderModelPayload } from "../types";
import { useRefreshAllModels } from "./use-refresh-all-models";

export interface RuntimeCatalogProvider {
  id: string;
  /** When the provider needs sign-in every one of its model rows is disabled. */
  needsAuth?: boolean;
}

export interface RuntimeModelCatalog {
  /**
   * Cross-provider `view=all` models mapped for the selector, restricted to the
   * providers the surface allows. The selector browses the curated subset and
   * searches the full set across those providers.
   */
  models: RuntimeModelOption[];
  /** Raw daemon payloads keyed by provider id (reasoning-source derivation). */
  payloadsByProvider: Record<string, ProviderModelPayload[]>;
  loading: boolean;
  /** The aggregate query has RESOLVED at least once (distinct from `!loading`). */
  loaded: boolean;
  error: string | null;
  /** Any allowed row is serving a stale source projection. */
  stale: boolean;
  /** Refresh the whole catalog (truthful global refresh for All/Favorites surfaces). */
  refresh: () => void;
  refreshing: boolean;
  refreshError: string | null;
}

function describeCatalogError(error: unknown): string {
  if (error instanceof Error && error.message.trim().length > 0) return error.message;
  return "Failed to load provider models.";
}

/**
 * Load the cross-provider catalog with a SINGLE aggregate `view=all` request
 * (`GET /api/model-catalog/models`) rather than fanning out per provider. Rows
 * are filtered down to the providers a surface allows — a workspace agent or
 * session flow can therefore never select a provider outside its workspace
 * provider set — and each row is disabled when its provider needs sign-in.
 * Browsing shows the curated subset while search reaches the full set; both span
 * every allowed provider per `_spec.md` §6.1 behavior rule 4. Refresh targets the
 * aggregate refresh endpoint so the visible affordance truthfully refreshes the
 * whole catalog instead of a single provider.
 */
export function useRuntimeModelCatalog(
  providers: readonly RuntimeCatalogProvider[],
  options: { enabled?: boolean; includeStale?: boolean } = {}
): RuntimeModelCatalog {
  const { enabled = true, includeStale = true } = options;
  const query = useQuery(allModelsListOptions({ view: "all", includeStale, enabled }));
  const refreshMutation = useRefreshAllModels();

  // Ordered allow-list: preserves the surface's provider order and carries each
  // provider's auth state so rows for a signed-out provider render disabled.
  const allowed = new Map<string, boolean>();
  for (const provider of providers) {
    const id = provider.id.trim();
    if (id.length > 0 && !allowed.has(id)) allowed.set(id, Boolean(provider.needsAuth));
  }

  const payloads = query.data?.models;

  const payloadsByProvider: Record<string, ProviderModelPayload[]> = {};
  for (const model of payloads ?? []) {
    if (!allowed.has(model.provider_id)) continue;
    const providerPayloads = payloadsByProvider[model.provider_id] ?? [];
    providerPayloads.push(model);
    payloadsByProvider[model.provider_id] = providerPayloads;
  }

  const models: RuntimeModelOption[] = [];
  for (const [providerId, needsAuth] of allowed) {
    const bucket = payloadsByProvider[providerId];
    if (!bucket) continue;
    models.push(...toRuntimeModelOptions(bucket, { providerNeedsAuth: needsAuth }));
  }

  const stale = Object.values(payloadsByProvider).some(bucket => bucket.some(model => model.stale));

  const refresh = () => refreshMutation.mutate();

  return {
    models,
    payloadsByProvider,
    loading: query.isLoading,
    loaded: query.isSuccess,
    error: query.error ? describeCatalogError(query.error) : null,
    stale,
    refresh,
    refreshing: refreshMutation.isPending,
    refreshError: refreshMutation.error ? describeCatalogError(refreshMutation.error) : null,
  };
}
