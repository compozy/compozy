import { parseBridgeProviderConfig } from "./bridge-drafts";
import { compactBridgeDeliveryDefaults, stableBridgeJSON } from "./bridge-formatters";
import type { BridgeUpdateDraft } from "../types";

/**
 * Whether a bridge update draft differs from what was loaded.
 *
 * `UpdateBridgeRequest` is an all-pointer patch and the daemon rejects a patch
 * with nothing in it, so the editor must not offer a save that cannot succeed.
 * Provider config is compared as parsed JSON rather than as text: reformatting
 * the same object is not a change the contract can carry.
 */
export function isBridgeUpdateDraftDirty(
  draft: BridgeUpdateDraft,
  pristine: BridgeUpdateDraft
): boolean {
  if (draft.displayName.trim() !== pristine.displayName.trim()) return true;
  if (draft.dmPolicy !== pristine.dmPolicy) return true;
  if (stableBridgeJSON(draft.routingPolicy) !== stableBridgeJSON(pristine.routingPolicy))
    return true;
  if (
    stableBridgeJSON(compactBridgeDeliveryDefaults(draft.deliveryDefaults)) !==
    stableBridgeJSON(compactBridgeDeliveryDefaults(pristine.deliveryDefaults))
  ) {
    return true;
  }

  const draftConfig = parseBridgeProviderConfig(draft.providerConfigText);
  const pristineConfig = parseBridgeProviderConfig(pristine.providerConfigText);
  // An unparseable draft is blocked by its own validation, not by this gate.
  if (draftConfig.error) return true;
  return stableBridgeJSON(draftConfig.value) !== stableBridgeJSON(pristineConfig.value);
}
