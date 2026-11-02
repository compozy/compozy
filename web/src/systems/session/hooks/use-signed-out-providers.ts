import { settingsProviderToOption, useSettingsProviders } from "@/systems/settings";

/** Providers that aren't signed in, by id: their agents can't start. */
export function useSignedOutProviders() {
  const providers = useSettingsProviders();
  const signedOut = new Map<string, string>();
  for (const provider of providers.data?.providers ?? []) {
    const option = settingsProviderToOption(provider);
    if (option.needs_auth) signedOut.set(option.id, option.name);
  }
  return signedOut;
}
