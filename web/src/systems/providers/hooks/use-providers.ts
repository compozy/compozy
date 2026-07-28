import { useQuery } from "@tanstack/react-query";

import {
  providerDetailOptions,
  providerListOptions,
  type ProviderDetailOptionsArgs,
} from "../lib/query-options";

export function useProviders() {
  return useQuery(providerListOptions());
}

export function useProvider(args: ProviderDetailOptionsArgs) {
  return useQuery(providerDetailOptions(args));
}
