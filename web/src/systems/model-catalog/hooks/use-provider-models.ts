import { useQuery } from "@tanstack/react-query";

import {
  providerModelStatusOptions,
  providerModelsListOptions,
  type ProviderModelStatusOptionsArgs,
  type ProviderModelsListOptionsArgs,
} from "../lib/query-options";

export function useProviderModels(args: ProviderModelsListOptionsArgs) {
  return useQuery(providerModelsListOptions(args));
}

export function useProviderModelStatus(args: ProviderModelStatusOptionsArgs) {
  return useQuery(providerModelStatusOptions(args));
}
