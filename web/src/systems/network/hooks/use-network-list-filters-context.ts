import { use } from "react";

import { NetworkListFiltersContext } from "../contexts/network-list-filters-context-value";
import type { UseNetworkListFiltersResult } from "./use-network-list-filters";

export function useNetworkListFiltersContext(): UseNetworkListFiltersResult {
  const ctx = use(NetworkListFiltersContext);
  if (!ctx) {
    throw new Error(
      "useNetworkListFiltersContext must be used inside <NetworkListFiltersProvider>"
    );
  }
  return ctx;
}
