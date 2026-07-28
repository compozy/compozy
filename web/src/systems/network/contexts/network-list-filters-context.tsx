import { useLayoutEffect, type ReactNode } from "react";
import { useStore } from "@xstate/store-react";

import { networkListFiltersLogic } from "../hooks/use-network-list-filters";
import { NetworkListFiltersContext } from "./network-list-filters-context-value";

interface NetworkListFiltersProviderProps {
  workspaceId: string;
  channel: string;
  children: ReactNode;
}

/** Supplies only a scoped mutable store; server state remains hook-owned. */
export function NetworkListFiltersProvider({
  workspaceId,
  channel,
  children,
}: NetworkListFiltersProviderProps) {
  const store = useStore(networkListFiltersLogic, { workspaceId, channel });
  useLayoutEffect(() => {
    store.trigger.scopeChanged({ workspaceId, channel });
  }, [store, workspaceId, channel]);

  return (
    <NetworkListFiltersContext.Provider value={store}>
      {children}
    </NetworkListFiltersContext.Provider>
  );
}
