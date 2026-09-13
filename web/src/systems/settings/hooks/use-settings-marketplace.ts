import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { marketplaceKeys } from "@/systems/marketplace";
import { updateSettingsMarketplace } from "../adapters/settings-marketplace-api";
import { settingsMarketplaceOptions } from "../lib/query-options";
import { settingsKeys } from "../lib/query-keys";
import {
  recordSettingsMutation,
  invalidateSettingsApplyRecords,
} from "./settings-mutation-helpers";

import type { SettingsUpdateMarketplaceRequest } from "../types";

export function useSettingsMarketplace() {
  return useQuery(settingsMarketplaceOptions());
}

export function useUpdateSettingsMarketplace() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (body: SettingsUpdateMarketplaceRequest) => updateSettingsMarketplace(body),
    onSuccess: async result => {
      recordSettingsMutation(result);
      await client.cancelQueries({ queryKey: marketplaceKeys.all });
      await Promise.all([
        client.invalidateQueries({ queryKey: settingsKeys.section("marketplace") }),
        client.invalidateQueries({ queryKey: marketplaceKeys.all }),
        invalidateSettingsApplyRecords(client),
      ]);
    },
  });
}
