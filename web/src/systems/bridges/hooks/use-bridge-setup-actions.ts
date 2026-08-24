import { useMutation, useQueryClient } from "@tanstack/react-query";

import { registerBridgeWebhook, sendBridgeTest, verifyBridge } from "../adapters/bridge-setup-api";
import { bridgeKeys } from "../lib/query-keys";
import type { SendBridgeTestRequest } from "../types";

interface BridgeSetupMutationParams {
  id: string;
  profile: string;
}

interface SendBridgeTestParams extends BridgeSetupMutationParams {
  data: SendBridgeTestRequest;
}

function invalidateBridgeDeliveryQueries(
  queryClient: ReturnType<typeof useQueryClient>,
  id: string
) {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: bridgeKeys.lists() }),
    // Scoped to this bridge but blind to lens: the delivery change is the same
    // fact under every one of them.
    queryClient.invalidateQueries({ queryKey: bridgeKeys.detailFor(id) }),
    queryClient.invalidateQueries({ queryKey: bridgeKeys.routesFor(id) }),
  ]);
}

export function useVerifyBridge() {
  // react-doctor-disable-next-line react-doctor/query-mutation-missing-invalidation -- Verification is a live read-only check and does not mutate bridge state.
  return useMutation({
    mutationFn: ({ id, profile }: BridgeSetupMutationParams) => verifyBridge(id, { profile }),
  });
}

export function useRegisterBridgeWebhook() {
  // react-doctor-disable-next-line react-doctor/query-mutation-missing-invalidation -- Registration returns a provider handshake and does not mutate cached bridge state.
  return useMutation({
    mutationFn: ({ id, profile }: BridgeSetupMutationParams) =>
      registerBridgeWebhook(id, { profile }),
  });
}

export function useSendBridgeTest() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ data, id, profile }: SendBridgeTestParams) =>
      sendBridgeTest(id, data, { profile }),
    onSettled: (_result, _error, { id }) => invalidateBridgeDeliveryQueries(queryClient, id),
  });
}
