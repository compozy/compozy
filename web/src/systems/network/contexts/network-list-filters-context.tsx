import type { ReactNode } from "react";

import type { UseNetworkListFiltersResult } from "../hooks/use-network-list-filters";
import { NetworkListFiltersContext } from "./network-list-filters-context-value";

interface NetworkListFiltersProviderProps {
  value: UseNetworkListFiltersResult;
  children: ReactNode;
}

export function NetworkListFiltersProvider({ value, children }: NetworkListFiltersProviderProps) {
  return (
    <NetworkListFiltersContext.Provider value={value}>
      {children}
    </NetworkListFiltersContext.Provider>
  );
}
