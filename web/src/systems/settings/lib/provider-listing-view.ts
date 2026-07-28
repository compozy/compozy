import { providerAuthSummary, providerStatusCopy } from "./provider-copy";
import { getProviderStateView } from "./provider-state";
import type { SettingsProviderEntry } from "../types";

export function providerListingView(provider: SettingsProviderEntry) {
  const state = getProviderStateView(provider);
  const modelCount = (provider.settings.models?.curated ?? []).length;
  return {
    state,
    status: providerStatusCopy(state, modelCount),
    authSummary: providerAuthSummary(provider),
    displayName: provider.settings.display_name?.trim() || provider.name,
    command: provider.settings.command?.trim() || provider.name,
    modelCount,
    actionLabel: state.cta.intent === "configure" ? "Set up" : "Edit",
  };
}
