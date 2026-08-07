import { useQuery } from "@tanstack/react-query";

import { extensionsListOptions } from "@/systems/extensions";

import {
  buildGatewayProviderModel,
  providerGeneration,
  type GatewayProviderCandidate,
  type GatewayProviderModel,
} from "../lib/gateway-provider-model";
import type { GatewayStatus, GatewayTier } from "../types";
import { useDisableGatewayProvider, useEnableGatewayProvider } from "./use-gateway-mutations";

export interface GatewayProvidersViewModel {
  isLoading: boolean;
  /** The inventory read failed; activations from gateway status still render. */
  inventoryError: string | null;
  actionError: string | null;
  isSubmitting: boolean;
  model: GatewayProviderModel;
  enable: (candidate: GatewayProviderCandidate, tier: GatewayTier) => void;
  disable: (name: string, tier: GatewayTier) => void;
}

/**
 * Connectivity providers, joined from durable gateway activations and the live
 * extension inventory.
 *
 * The inventory is read at the **global** instance: a connectivity provider is
 * an operator-wide capability, and the daemon refuses workspace-scoped sources
 * outright, so scoping this read to the active workspace would offer providers
 * that can never be authorized.
 */
export function useGatewayProviders(status: GatewayStatus | null): GatewayProvidersViewModel {
  const inventory = useQuery(extensionsListOptions());
  const enableProvider = useEnableGatewayProvider();
  const disableProvider = useDisableGatewayProvider();
  const model = buildGatewayProviderModel(status ?? emptyStatus, inventory.data ?? []);

  return {
    isLoading: inventory.isLoading,
    inventoryError: inventory.error?.message ?? null,
    actionError: (enableProvider.error ?? disableProvider.error)?.message ?? null,
    isSubmitting: enableProvider.isPending || disableProvider.isPending,
    model,
    enable: (candidate, tier) =>
      enableProvider.mutate({
        name: candidate.name,
        tier,
        installSource: candidate.installSource,
        expectedGeneration: providerGeneration(candidate, tier),
        ...(candidate.digest ? { digest: candidate.digest } : {}),
      }),
    disable: (name, tier) => disableProvider.mutate({ name, tier }),
  };
}

const emptyStatus = {
  addresses: [],
  bindings: [],
  changed: false,
  devices: [],
  enabled: false,
  providers: [],
  refusal: null,
  surfaces: [],
  tiers: [],
} satisfies GatewayStatus;
