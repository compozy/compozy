import { useEffect, useRef } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";

import { clearRemoteApiClients } from "@/lib/api-client";
import { evictRemoteComputerQueries } from "@/lib/computer-scope";
import { accountKeys, accountStatusOptions, openAccountStream } from "@/systems/account";

import { computerRegistryKeys } from "../lib/query-keys";

/**
 * Evicts all remote `["computer", id, ...]` queries and refreshes the controller's computer list
 * whenever the signed-in account signs out or switches (Rule 23, US-001.EC-4, IT-026).
 */
export function useAccountRemoteEviction(enabled = true) {
  const queryClient = useQueryClient();
  const accountQuery = useQuery(accountStatusOptions(enabled));
  const previousIdentityRef = useRef<string | null | undefined>(undefined);

  const accountIdentity = accountQuery.data
    ? accountQuery.data.signed_in
      ? (accountQuery.data.email ?? accountQuery.data.device?.id ?? "signed_in")
      : null
    : undefined;

  useEffect(() => {
    if (!enabled || accountIdentity === undefined) return;
    const previous = previousIdentityRef.current;
    previousIdentityRef.current = accountIdentity;
    if (previous !== undefined && previous !== accountIdentity) {
      clearRemoteApiClients();
      evictRemoteComputerQueries(queryClient, { includeComputers: true });
      void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.all });
    }
  }, [accountIdentity, enabled, queryClient]);

  useEffect(() => {
    if (!enabled) return;
    return openAccountStream({
      onAccountChanged: event => {
        if (event.evict_remote) {
          clearRemoteApiClients();
          evictRemoteComputerQueries(queryClient, { includeComputers: true });
        }
        void queryClient.invalidateQueries({ queryKey: accountKeys.all });
        void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.all });
      },
      onReset: () => {
        void queryClient.invalidateQueries({ queryKey: accountKeys.all });
        void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.all });
      },
    });
  }, [enabled, queryClient]);
}
