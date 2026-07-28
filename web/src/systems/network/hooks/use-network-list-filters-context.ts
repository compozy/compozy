import { use } from "react";

import { NetworkListFiltersContext } from "../contexts/network-list-filters-context-value";
import type { NetworkListFiltersStore } from "./use-network-list-filters";

export function useNetworkListFiltersStore(): NetworkListFiltersStore {
  const store = use(NetworkListFiltersContext);
  if (!store) {
    throw new Error("useNetworkListFiltersStore must be used inside <NetworkListFiltersProvider>");
  }
  return store;
}
