import { createStore } from "@xstate/store";

import { observeGatewayAccess, type GatewayAccessSignal } from "@/lib/gateway-access-signal";

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
  context: { state: "ok" as GatewayAccessState },
  on: {
    accessSignalled: (context, event: { signal: GatewayAccessSignal }) => {
      if (context.state === event.signal) return undefined;
      if (context.state === "revoked") return undefined;
      return { ...context, state: event.signal };
    },
    accessRestored: context => {
      if (context.state === "ok") return undefined;
      return { ...context, state: "ok" as GatewayAccessState };
    },
  },
});

let unobserve: (() => void) | null = null;

/**
 * Binds the store to the shared transport. Idempotent so several mounts (or a
 * StrictMode double-invoke) keep exactly one subscription.
 */
export function startGatewayAccessObserver(): () => void {
  unobserve ??= observeGatewayAccess(signal =>
    gatewayAccessStore.trigger.accessSignalled({ signal })
  );
  return () => {
    unobserve?.();
    unobserve = null;
  };
}
