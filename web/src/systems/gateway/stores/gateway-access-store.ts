import { createStore } from "@xstate/store";

import {
  observeGatewayAccess,
  observeGatewayListenerTier,
  type GatewayAccessSignal,
  type GatewayListenerTier,
} from "@/lib/gateway-access-signal";

export type GatewayAccessState = "ok" | "unauthenticated" | "revoked";

/**
 * Whether this browser still holds a device session.
 *
 * App-wide and module-scoped, because the answer is a property of the page's
 * connection rather than of any component. It moves only on an explicit daemon
 * decision (see `@/lib/gateway-access-signal`) — never on a dropped connection,
 * a timeout, or a 5xx — so a bad link cannot eject the operator.
 *
 * `revoked` is terminal against `unauthenticated`: once the daemon says this
 * device was revoked, a later generic 401 must not soften the message the
 * person is reading.
 */
export const gatewayAccessStore = createStore({
  context: {
    state: "ok" as GatewayAccessState,
    tier: undefined as GatewayListenerTier | undefined,
  },
  on: {
    accessSignalled: (context, event: { signal: GatewayAccessSignal }) => {
      if (context.state === event.signal) return undefined;
      if (context.state === "revoked") return undefined;
      return { ...context, state: event.signal };
    },
    accessRestored: context => {
      if (context.state === "ok" && context.tier === undefined) return undefined;
      return { ...context, state: "ok" as GatewayAccessState, tier: undefined };
    },
    tierSignalled: (context, event: { tier: GatewayListenerTier }) => {
      if (context.tier === event.tier) return undefined;
      return { ...context, tier: event.tier };
    },
  },
});

let unobserve: (() => void) | null = null;
let observerLeases = 0;

/**
 * Binds the store to the shared transport. Idempotent so several mounts (or a
 * StrictMode double-invoke) keep exactly one subscription.
 */
export function startGatewayAccessObserver(): () => void {
  if (observerLeases === 0) {
    const stopAccess = observeGatewayAccess(signal =>
      gatewayAccessStore.trigger.accessSignalled({ signal })
    );
    const stopTier = observeGatewayListenerTier(tier =>
      gatewayAccessStore.trigger.tierSignalled({ tier })
    );
    unobserve = () => {
      stopAccess();
      stopTier();
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
