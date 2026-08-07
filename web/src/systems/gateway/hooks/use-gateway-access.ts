import { useEffect } from "react";
import { useMutation } from "@tanstack/react-query";
import { useSelector } from "@xstate/store-react";

import { gatewayApi } from "../adapters/gateway-api";
import {
  gatewayAccessStore,
  startGatewayAccessObserver,
  type GatewayAccessState,
} from "../stores/gateway-access-store";
import type { GatewayRedeemResult } from "../types";

/**
 * The page's device standing. Subscribing here also binds the store to the
 * shared transport — an external-system subscription, which is what `useEffect`
 * is for.
 */
export function useGatewayAccessState(): GatewayAccessState {
  useEffect(() => startGatewayAccessObserver(), []);
  return useSelector(gatewayAccessStore, snapshot => snapshot.context.state);
}

export interface RedeemPairingInput {
  artifact: string;
  name: string;
}

/**
 * Redeems a pairing artifact from an unpaired browser. The daemon answers with
 * a session cookie and nothing the page needs to hold, so the app is reloaded
 * rather than patched: every query, stream and window-manager socket has to be
 * re-established under the new device session, and a reload is the only way to
 * guarantee none of them is left running under the old one.
 */
export function useRedeemGatewayPairing(reload: () => void = defaultReload) {
  return useMutation<GatewayRedeemResult, Error, RedeemPairingInput>({
    mutationFn: ({ artifact, name }) =>
      gatewayApi.redeemPairing({ actor_kind: "operator_device", artifact, name }),
    onSuccess: () => {
      gatewayAccessStore.trigger.accessRestored();
      reload();
    },
  });
}

function defaultReload(): void {
  if (typeof window === "undefined") return;
  window.location.reload();
}
