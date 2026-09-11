// Imported from the store module, not the barrel: test files that mock other
// systems' modules must not pull the full gateway module graph into the file,
// or the barrel's transitive imports race with the file's `vi.mock` factories.
import { gatewayCapabilityStore } from "@/systems/gateway/stores/gateway-capability-store";
import type { GatewayListenerTier } from "@/lib/gateway-access-signal";

/**
 * Latches the app-wide capability store for a test and returns the reset.
 *
 * Production latches the tier from every `/api` response; component tests that
 * render tier-gated consumers without the transport latch `local` here so the
 * suite keeps asserting the local shape. Tests asserting remote behavior latch
 * a remote tier (or nothing — the unlatched default is the all-false set).
 */
export function latchGatewayTierForTest(tier: GatewayListenerTier = "local"): () => void {
  gatewayCapabilityStore.trigger.tierSignalled({ tier });
  return () => gatewayCapabilityStore.trigger.capabilitiesReset();
}
