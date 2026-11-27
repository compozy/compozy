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

type SettingsProviderUsageWrite =
  | SettingsProviderUsageSave
  | { consentProviderId: string; collectionProfile: string };

/**
 * Saves the whole provider usage config through the apply lifecycle. Usage
 * views re-read too: an apply can remove targets or turn the feature off.
 */
export function useUpdateSettingsProviderUsage() {
  const queryClient = useQueryClient();
  return useMutation({
    networkMode: "always",
    scope: { id: "settings-provider-usage" },
    mutationFn: async (input: SettingsProviderUsageWrite) => {
      if ("body" in input) {
        return updateSettingsProviderUsage(input.body, input.collectionProfile);
      }
      // Read after the preceding save so two Allow actions preserve both grants.
      const { config } = await queryClient.fetchQuery({
        ...settingsProviderUsageOptions(),
        staleTime: 0,
      });
      return updateSettingsProviderUsage(
        { config: withProviderUsageKeychainAccess(config, input.consentProviderId, true) },
        input.collectionProfile
      );
    },
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
 * Saves the usage row's Keychain grant and reports a failed save.
 * The daemon collects for the active profile after the grant applies.
 */
export function useAllowProviderUsageKeychain(
  profile: string
): (providerId: string) => Promise<void> {
  const mutation = useUpdateSettingsProviderUsage();
  return async providerId => {
    try {
      await mutation.mutateAsync({ consentProviderId: providerId, collectionProfile: profile });
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
