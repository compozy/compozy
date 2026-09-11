import { createStore } from "@xstate/store";

import {
  observeGatewayForbiddenResponse,
  observeGatewayListenerTier,
  type GatewayListenerTier,
} from "@/lib/gateway-access-signal";

import { capabilitiesForTier, type GatewayCapabilities } from "../lib/gateway-capabilities";
import { classifyGatewayLoopback, type GatewayLoopbackSignal } from "../lib/gateway-loopback";

export interface GatewayCapabilityState {
  /** Latched listener tier; `undefined` until an observation carries a parsable tier. */
  tier: GatewayListenerTier | undefined;
  /** Derived from the tier alone; re-derived whenever the tier latches or changes. */
  capabilities: GatewayCapabilities;
  /**
   * The most recent loopback-only 403 classification, if any — the backstop
   * evidence that the daemon refused a call this listener cannot execute
   * (US-002). Cleared when the tier latches `local`, where the same call
   * would succeed; a retry on a remote tier re-sets it (US-002.EC-1).
   */
  loopbackOnly: GatewayLoopbackSignal | undefined;
}

/**
 * What this browser's listener can execute, plus the loopback backstop.
 *
 * App-wide and module-scoped, like the access store: the tier is a property of
 * the page's connection, not of any component. Capabilities derive from the
 * latched tier only (no per-route discovery), so every tier observation
 * re-evaluates visibility without an app restart (BR-4).
 */
export const gatewayCapabilityStore = createStore({
  context: {
    tier: undefined as GatewayListenerTier | undefined,
    capabilities: capabilitiesForTier(undefined),
    loopbackOnly: undefined as GatewayLoopbackSignal | undefined,
  },
  on: {
    tierSignalled: (context, event: { tier: GatewayListenerTier }) => {
      const capabilities = capabilitiesForTier(event.tier);
      // A `local` latch makes any recorded loopback refusal stale: on a
      // loopback-bound listener the refused call would succeed. Remote-to-
      // remote changes keep it — the daemon-host guidance still holds.
      const loopbackOnly = event.tier === "local" ? undefined : context.loopbackOnly;
      if (
        context.tier === event.tier &&
        context.capabilities === capabilities &&
        context.loopbackOnly === loopbackOnly
      ) {
        return undefined;
      }
      return { ...context, tier: event.tier, capabilities, loopbackOnly };
    },
    loopbackRefused: (context, event: { signal: GatewayLoopbackSignal }) => {
      // Re-derive from the latched tier: `reportGatewayResponse` latched the
      // refusing response's tier header first, so a stale capability map
      // converges from the same response (US-002.EC-2).
      const capabilities = capabilitiesForTier(context.tier);
      if (context.loopbackOnly === event.signal && context.capabilities === capabilities) {
        return undefined;
      }
      return { ...context, loopbackOnly: event.signal, capabilities };
    },
    // Returns the store to its unlatched initial state. The access boundary
    // fires this when this browser's device standing resets (a restored
    // session re-latches the tier from the next response); tests use it to
    // start each case unlatched.
    capabilitiesReset: context => {
      if (
        context.tier === undefined &&
        context.capabilities === capabilitiesForTier(undefined) &&
        context.loopbackOnly === undefined
      ) {
        return undefined;
      }
      return {
        tier: undefined,
        capabilities: capabilitiesForTier(undefined),
        loopbackOnly: undefined,
      };
    },
  },
});

let unobserve: (() => void) | null = null;
let observerLeases = 0;

/**
 * Binds the store to the shared transport. Idempotent so several mounts (or a
 * StrictMode double-invoke) keep exactly one subscription.
 */
export function startGatewayCapabilityObserver(): () => void {
  if (observerLeases === 0) {
    const stopTier = observeGatewayListenerTier(tier =>
      gatewayCapabilityStore.trigger.tierSignalled({ tier })
    );
    const stopForbidden = observeGatewayForbiddenResponse((status, payload) => {
      const signal = classifyGatewayLoopback(status, payload);
      if (!signal) return;
      gatewayCapabilityStore.trigger.loopbackRefused({ signal });
    });
    unobserve = () => {
      stopTier();
      stopForbidden();
    };
  }
  observerLeases += 1;
  let released = false;
  return () => {
    if (released) return;
    released = true;
    observerLeases -= 1;
    if (observerLeases !== 0) return;
    unobserve?.();
    unobserve = null;
  };
}
