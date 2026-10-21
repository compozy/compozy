import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { toast } from "@compozy/ui";
import { providerUsageKeys } from "@/systems/provider-usage";

import { updateSettingsProviderUsage } from "../adapters/settings-api";
import { withProviderUsageKeychainAccess } from "../lib/provider-usage-settings";
import { settingsKeys } from "../lib/query-keys";
import { settingsProviderUsageOptions } from "../lib/query-options";
import type { SettingsUpdateProviderUsageRequest } from "../types";
import {
  invalidateSettingsApplyRecords,
  recordSettingsMutation,
} from "./settings-mutation-helpers";

export function useSettingsProviderUsage() {
  return useQuery(settingsProviderUsageOptions());
}

export interface SettingsProviderUsageSave {
  body: SettingsUpdateProviderUsageRequest;
  /** The shell's active profile: a consent change collects for it, never `default`. */
  collectionProfile: string;
}

/**
 * Saves the whole provider usage config through the apply lifecycle. Usage
 * views re-read too: an apply can remove targets or turn the feature off.
 */
export function useUpdateSettingsProviderUsage() {
  const queryClient = useQueryClient();
  return useMutation({
    networkMode: "always",
    mutationFn: ({ body, collectionProfile }: SettingsProviderUsageSave) =>
      updateSettingsProviderUsage(body, collectionProfile),
    onSuccess: recordSettingsMutation,
    onSettled: () =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: settingsKeys.section("provider-usage") }),
        invalidateSettingsApplyRecords(queryClient),
        queryClient.invalidateQueries({ queryKey: providerUsageKeys.all }),
      ]),
  });
}

/**
 * The usage row's **Allow**: saves `provider_usage.providers.claude.keychain_access
 * = true` over the current config for `profile` — the profile the follow-up
 * refresh targets — and reports a failed save itself. The returned promise
 * rejects on failure so the caller sends no refresh.
 */
export function useAllowProviderUsageKeychain(
  profile: string
): (providerId: string) => Promise<void> {
  const queryClient = useQueryClient();
  const mutation = useUpdateSettingsProviderUsage();
  return async () => {
    try {
      const section = await queryClient.fetchQuery(settingsProviderUsageOptions());
      await mutation.mutateAsync({
        body: { config: withProviderUsageKeychainAccess(section.config, true) },
        collectionProfile: profile,
      });
    } catch (error) {
      toast.error(
        error instanceof Error && error.message
          ? `Couldn't allow Keychain access: ${error.message}`
          : "Couldn't allow Keychain access"
      );
      throw error;
    }
  };
}
