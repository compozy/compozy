import { useMutation, useQueryClient } from "@tanstack/react-query";

import { gatewayApi } from "../adapters/gateway-api";
import { gatewayKeys } from "../lib/query-keys";
import type {
  GatewayDevice,
  GatewayPairingArtifact,
  GatewayRevokeResult,
  GatewayStatus,
  GatewaySurfaceRequest,
  GatewayTier,
} from "../types";

/**
 * Every exposure transition returns the reconciled status, so the cache is
 * reconciled from the daemon's own projection instead of an optimistic guess —
 * a transition can be refused, fenced by generation, or land degraded, and the
 * surface must show what actually happened.
 */
export function useSetGatewaySurface() {
  const queryClient = useQueryClient();
  return useMutation<GatewayStatus, Error, GatewaySurfaceRequest>({
    mutationFn: body => gatewayApi.setSurface(body),
    onSuccess: status => {
      queryClient.setQueryData(gatewayKeys.status(), status);
      void queryClient.invalidateQueries({ queryKey: gatewayKeys.audit() });
    },
  });
}

export interface EnableGatewayProviderInput {
  name: string;
  tier: GatewayTier;
  /** Re-derived by the daemon from the live registry; sent for the fence, not as truth. */
  installSource: string;
  digest?: string;
  expectedGeneration: number;
}

/**
 * Activates one installed connectivity provider on one tier.
 *
 * The daemon re-derives install source and control digest from the live
 * extension registry before it authorizes anything, so a refusal here is the
 * authoritative answer and is surfaced verbatim rather than second-guessed.
 */
export function useEnableGatewayProvider() {
  const queryClient = useQueryClient();
  return useMutation<GatewayStatus, Error, EnableGatewayProviderInput>({
    mutationFn: input =>
      gatewayApi.enableProvider(input.name, {
        expected_generation: input.expectedGeneration,
        install_source: input.installSource,
        tier: input.tier,
        ...(input.digest ? { digest_confirmed: input.digest } : {}),
      }),
    onSuccess: status => {
      queryClient.setQueryData(gatewayKeys.status(), status);
      void queryClient.invalidateQueries({ queryKey: gatewayKeys.audit() });
    },
  });
}

export function useDisableGatewayProvider() {
  const queryClient = useQueryClient();
  return useMutation<GatewayStatus, Error, { name: string; tier: GatewayTier }>({
    mutationFn: ({ name, tier }) => gatewayApi.disableProvider(name, tier),
    onSuccess: status => {
      queryClient.setQueryData(gatewayKeys.status(), status);
      void queryClient.invalidateQueries({ queryKey: gatewayKeys.audit() });
    },
  });
}

/**
 * The artifact is returned once and never again. It is deliberately not cached:
 * a re-mount must re-mint rather than resurrect a code the daemon may already
 * have marked spent or expired.
 */
export function useMintGatewayPairing() {
  const queryClient = useQueryClient();
  return useMutation<GatewayPairingArtifact, Error, void>({
    mutationFn: () => gatewayApi.mintPairing(),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: gatewayKeys.devices() });
    },
  });
}

export function useRenameGatewayDevice() {
  const queryClient = useQueryClient();
  return useMutation<GatewayDevice, Error, { id: string; name: string }>({
    mutationFn: ({ id, name }) => gatewayApi.renameDevice(id, name),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: gatewayKeys.devices() });
      void queryClient.invalidateQueries({ queryKey: gatewayKeys.status() });
    },
  });
}

/**
 * Revocation cancels the device's live streams before it returns, so the
 * inventory and the posture are both reread rather than patched locally.
 */
export function useRevokeGatewayDevice() {
  const queryClient = useQueryClient();
  return useMutation<GatewayRevokeResult, Error, string>({
    mutationFn: id => gatewayApi.revokeDevice(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: gatewayKeys.devices() });
      void queryClient.invalidateQueries({ queryKey: gatewayKeys.status() });
      void queryClient.invalidateQueries({ queryKey: gatewayKeys.audit() });
    },
  });
}
