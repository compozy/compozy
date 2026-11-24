import { useQuery } from "@tanstack/react-query";

import {
  providerDetailOptions,
  providerListOptions,
  type ProviderDetailOptionsArgs,
  type ProviderListOptionsArgs,
} from "../lib/query-options";

export function useProviders(args?: ProviderListOptionsArgs) {
  return useQuery(providerListOptions(args));
}

export function useProvider(args: ProviderDetailOptionsArgs) {
  return useQuery(providerDetailOptions(args));
}
