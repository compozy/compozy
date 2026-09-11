import { useEffect } from "react";
import { useSelector } from "@xstate/store-react";

import {
  startGatewayCapabilityObserver,
  gatewayCapabilityStore,
} from "../stores/gateway-capability-store";
import type { GatewayCapabilities } from "../lib/gateway-capabilities";
import type { GatewayLoopbackSignal } from "../lib/gateway-loopback";

/**
 * What this browser's listener can execute, per the latched tier. Subscribing
 * here also binds the store to the shared transport — an external-system
 * subscription, which is what `useEffect` is for. Until the tier latches, the
 * all-false set applies: loopback-only affordances stay hidden (BR-2), and a
 * tier change re-evaluates without a restart (BR-4).
 */
export function useGatewayCapabilities(): GatewayCapabilities {
  useEffect(() => startGatewayCapabilityObserver(), []);
  return useSelector(gatewayCapabilityStore, snapshot => snapshot.context.capabilities);
}

/**
 * The most recent loopback-only 403 classification, if any — the backstop
 * evidence behind the truthful loopback-only state (US-002).
 */
export function useGatewayLoopbackOnly(): GatewayLoopbackSignal | undefined {
  useEffect(() => startGatewayCapabilityObserver(), []);
  return useSelector(gatewayCapabilityStore, snapshot => snapshot.context.loopbackOnly);
}
