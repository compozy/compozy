import { useMutation, useQueryClient } from "@tanstack/react-query";

import { useProfileReadScope } from "@/systems/profiles";

import {
  createBridge,
  deleteBridgeSecretBinding,
  disableBridge,
  enableBridge,
  putBridgeSecretBinding,
  restartBridge,
  resolveBridgeTarget,
  testBridgeDelivery,
  updateBridge,
} from "../adapters/bridges-api";
import { bridgeKeys } from "../lib/query-keys";
import type {
  CreateBridgeRequest,
  BridgeResolveTargetRequest,
  PutBridgeSecretBindingRequest,
  TestBridgeDeliveryRequest,
  UpdateBridgeRequest,
} from "../types";

interface BridgeIdParams {
  id: string;
}

interface OwnedBridgeMutationParams extends BridgeIdParams {
  profile: string;
}

interface TestBridgeDeliveryParams extends OwnedBridgeMutationParams {
  data: TestBridgeDeliveryRequest;
}

interface ResolveBridgeTargetParams extends BridgeIdParams {
  data: BridgeResolveTargetRequest;
}

interface UpdateBridgeParams extends OwnedBridgeMutationParams {
  data: UpdateBridgeRequest;
}

interface PutBridgeSecretBindingParams extends OwnedBridgeMutationParams {
  bindingName: string;
  data: PutBridgeSecretBindingRequest;
}

interface DeleteBridgeSecretBindingParams extends OwnedBridgeMutationParams {
  bindingName: string;
}

function invalidateBridgeQueries(
  queryClient: ReturnType<typeof useQueryClient>,
  id?: string,
  options?: {
    includeRoutes?: boolean;
    includeSecretBindings?: boolean;
  }
) {
  const includeRoutes = options?.includeRoutes ?? false;
  const includeSecretBindings = options?.includeSecretBindings ?? false;

  return Promise.all([
    queryClient.invalidateQueries({ queryKey: bridgeKeys.all }),
    ...(id
      ? [
          // The lens-blind prefix: a mutation changes the bridge itself, so
          // every lens holding it must reread, not only the active one.
          queryClient.invalidateQueries({ queryKey: bridgeKeys.detailFor(id) }),
          ...(includeRoutes
            ? [queryClient.invalidateQueries({ queryKey: bridgeKeys.routesFor(id) })]
            : []),
          ...(includeSecretBindings
            ? [queryClient.invalidateQueries({ queryKey: bridgeKeys.secretBindingsFor(id) })]
            : []),
        ]
      : []),
  ]);
}

export function useCreateBridge() {
  const queryClient = useQueryClient();
  // The bridge lands in the acting profile — `default` while the aggregate is
  // on, which is exactly what the destination chip states (ADR-005).
  const { destination } = useProfileReadScope();

  return useMutation({
    mutationFn: (data: CreateBridgeRequest) => createBridge(data, destination),
    onSettled: () => invalidateBridgeQueries(queryClient),
  });
}

export function useTestBridgeDelivery() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, data, profile }: TestBridgeDeliveryParams) =>
      testBridgeDelivery(id, data, { profile }),
    onSettled: (_result, _error, { id }) =>
      invalidateBridgeQueries(queryClient, id, { includeRoutes: true }),
  });
}

export function useResolveBridgeTarget() {
  const queryClient = useQueryClient();
  const { params } = useProfileReadScope();

  return useMutation({
    mutationFn: ({ id, data }: ResolveBridgeTargetParams) => resolveBridgeTarget(id, data, params),
    onSuccess: (_result, { id }) =>
      queryClient.invalidateQueries({ queryKey: bridgeKeys.targetsForBridge(id) }),
  });
}

export function useUpdateBridge() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, data, profile }: UpdateBridgeParams) => updateBridge(id, data, { profile }),
    onSettled: (_result, _error, { id }) =>
      invalidateBridgeQueries(queryClient, id, { includeRoutes: true }),
  });
}

export function usePutBridgeSecretBinding() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ bindingName, data, id, profile }: PutBridgeSecretBindingParams) =>
      putBridgeSecretBinding(id, bindingName, data, { profile }),
    onSettled: (_result, _error, { id }) =>
      invalidateBridgeQueries(queryClient, id, { includeSecretBindings: true }),
  });
}

export function useDeleteBridgeSecretBinding() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ bindingName, id, profile }: DeleteBridgeSecretBindingParams) =>
      deleteBridgeSecretBinding(id, bindingName, { profile }),
    onSettled: (_result, _error, { id }) =>
      invalidateBridgeQueries(queryClient, id, { includeSecretBindings: true }),
  });
}

function useBridgeLifecycleMutation<TResult>(
  mutationFn: (id: string, scope: { profile: string }) => Promise<TResult>
) {
  const queryClient = useQueryClient();

  return useMutation<TResult, Error, OwnedBridgeMutationParams>({
    mutationFn: ({ id, profile }) => mutationFn(id, { profile }),
    onSettled: (_result, _error, { id }) =>
      invalidateBridgeQueries(queryClient, id, {
        includeRoutes: true,
        includeSecretBindings: true,
      }),
  });
}

export function useEnableBridge() {
  return useBridgeLifecycleMutation(enableBridge);
}

export function useDisableBridge() {
  return useBridgeLifecycleMutation(disableBridge);
}

export function useRestartBridge() {
  return useBridgeLifecycleMutation(restartBridge);
}
