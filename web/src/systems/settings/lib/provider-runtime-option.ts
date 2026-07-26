import { providerNeedsAuth } from "@/systems/model-catalog";
import type { RuntimeProviderOption } from "@/systems/runtime";

import type { SettingsProviderEntry } from "../types";

/**
 * Adapt a configured settings provider into a runtime-selector option. Settings
 * owns `SettingsProviderEntry`, so every surface that drives a `RuntimeSelector`
 * from the global provider list reads this one mapper.
 */
export function settingsProviderToOption(provider: SettingsProviderEntry): RuntimeProviderOption {
  const displayName = provider.settings.display_name?.trim();
  const harness = provider.settings.harness?.trim();
  const runtimeProvider = provider.settings.runtime_provider?.trim();
  return {
    id: provider.name,
    name: displayName || provider.name,
    ...(harness ? { harness } : {}),
    ...(runtimeProvider ? { runtime_provider: runtimeProvider } : {}),
    needs_auth: providerNeedsAuth(provider.auth_status?.state),
  };
}
