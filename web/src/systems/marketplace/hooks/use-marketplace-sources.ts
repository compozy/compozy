import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  addMarketplaceSource,
  updateMarketplaceSource,
  removeMarketplaceSource,
  refreshMarketplaceSource,
} from "../adapters/marketplace-sources-api";
import { marketplaceSourcesOptions } from "../lib/query-options";
import { invalidateMarketplace } from "./use-marketplace-actions";
import type { AddMarketplaceSourceRequest, UpdateMarketplaceSourceRequest } from "../types";

export function useMarketplaceSources() {
  return useQuery(marketplaceSourcesOptions());
}

export function useAddMarketplaceSource() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (body: AddMarketplaceSourceRequest) => addMarketplaceSource(body),
    onSettled: () => invalidateMarketplace(client),
  });
}

export function useUpdateMarketplaceSource() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: ({ name, body }: { name: string; body: UpdateMarketplaceSourceRequest }) =>
      updateMarketplaceSource(name, body),
    onSettled: () => invalidateMarketplace(client),
  });
}

export function useRemoveMarketplaceSource() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => removeMarketplaceSource(name),
    onSettled: () => invalidateMarketplace(client),
  });
}

export function useRefreshMarketplaceSource() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => refreshMarketplaceSource(name),
    onSettled: () => invalidateMarketplace(client),
  });
}
