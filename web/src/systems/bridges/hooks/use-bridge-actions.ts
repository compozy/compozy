import { useMutation, useQueryClient } from "@tanstack/react-query";

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

interface BridgeMutationIdParams {
  id: string;
}

interface TestBridgeDeliveryParams extends BridgeMutationIdParams {
  data: TestBridgeDeliveryRequest;
}

interface ResolveBridgeTargetParams extends BridgeMutationIdParams {
  data: BridgeResolveTargetRequest;
}

interface UpdateBridgeParams extends BridgeMutationIdParams {
  data: UpdateBridgeRequest;
}

interface PutBridgeSecretBindingParams extends BridgeMutationIdParams {
  bindingName: string;
  data: PutBridgeSecretBindingRequest;
}

interface DeleteBridgeSecretBindingParams extends BridgeMutationIdParams {
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
          queryClient.invalidateQueries({ queryKey: bridgeKeys.detail(id) }),
          ...(includeRoutes
            ? [queryClient.invalidateQueries({ queryKey: bridgeKeys.routes(id) })]
            : []),
          ...(includeSecretBindings
            ? [queryClient.invalidateQueries({ queryKey: bridgeKeys.secretBindings(id) })]
            : []),
        ]
      : []),
  ]);
}

export function useCreateBridge() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateBridgeRequest) => createBridge(data),
    onSettled: () => invalidateBridgeQueries(queryClient),
  });
}

export function useTestBridgeDelivery() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, data }: TestBridgeDeliveryParams) => testBridgeDelivery(id, data),
    onSettled: (_result, _error, { id }) =>
      invalidateBridgeQueries(queryClient, id, { includeRoutes: true }),
  });
}

export function useResolveBridgeTarget() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, data }: ResolveBridgeTargetParams) => resolveBridgeTarget(id, data),
    onSuccess: (_result, { id }) =>
      queryClient.invalidateQueries({ queryKey: bridgeKeys.targetsForBridge(id) }),
  });
}

export function useUpdateBridge() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, data }: UpdateBridgeParams) => updateBridge(id, data),
    onSettled: (_result, _error, { id }) =>
      invalidateBridgeQueries(queryClient, id, { includeRoutes: true }),
  });
}

export function usePutBridgeSecretBinding() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ bindingName, data, id }: PutBridgeSecretBindingParams) =>
      putBridgeSecretBinding(id, bindingName, data),
    onSettled: (_result, _error, { id }) =>
      invalidateBridgeQueries(queryClient, id, { includeSecretBindings: true }),
  });
}

export function useDeleteBridgeSecretBinding() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ bindingName, id }: DeleteBridgeSecretBindingParams) =>
      deleteBridgeSecretBinding(id, bindingName),
    onSettled: (_result, _error, { id }) =>
      invalidateBridgeQueries(queryClient, id, { includeSecretBindings: true }),
  });
}

function useBridgeLifecycleMutation<TResult>(mutationFn: (id: string) => Promise<TResult>) {
  const queryClient = useQueryClient();

  return useMutation<TResult, Error, BridgeMutationIdParams>({
    mutationFn: ({ id }: BridgeMutationIdParams) => mutationFn(id),
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
