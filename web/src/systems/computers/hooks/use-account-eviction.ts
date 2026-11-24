import { useEffect, useRef } from "react";
import { useQuery, useQueryClient, type QueryClient } from "@tanstack/react-query";

import { clearRemoteApiClients } from "@/lib/api-client";
import { evictRemoteComputerQueries } from "@/lib/computer-scope";
import { isHostedBrowserMode } from "@/lib/hosted-browser-auth";
import {
  accountKeys,
  accountStatusOptions,
  openAccountStream,
  type AccountChangedEventData,
  type AccountStatusPayload,
} from "@/systems/account";

import { computerRegistryKeys } from "../lib/query-keys";

function accountIdentityKey(data: AccountStatusPayload | undefined): string | null | undefined {
  if (!data) return undefined;
  if (!data.signed_in) return null;
  return data.email ?? data.device?.id ?? "signed_in";
}

function handleAccountChangedEvent(queryClient: QueryClient, event: AccountChangedEventData): void {
  if (event.evict_remote) {
    clearRemoteApiClients();
    evictRemoteComputerQueries(queryClient, { includeComputers: true });
  }
  void queryClient.invalidateQueries({ queryKey: accountKeys.all });
  if (!event.evict_remote) {
    void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.all });
  }
}

/**
 * Evicts all remote `["computer", id, ...]` queries and refreshes the controller's computer list
 * whenever the signed-in account signs out or switches (Rule 23, US-001.EC-4, IT-026).
 */
export function useAccountRemoteEviction(enabled = true): { signedIn: boolean } {
  const active = enabled && !isHostedBrowserMode();
  const queryClient = useQueryClient();
  const accountQuery = useQuery(accountStatusOptions(active));
  const previousIdentityRef = useRef<string | null | undefined>(undefined);
  const signedIn = active && accountQuery.data?.signed_in === true;
  const accountIdentity = accountIdentityKey(accountQuery.data);

  useEffect(() => {
    if (!active || accountIdentity === undefined) return;
    const previous = previousIdentityRef.current;
    previousIdentityRef.current = accountIdentity;
    if (previous === undefined || previous === accountIdentity) return;
    clearRemoteApiClients();
    evictRemoteComputerQueries(queryClient, { includeComputers: true });
    if (accountIdentity !== null) {
      void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.all });
    }
  }, [accountIdentity, active, queryClient]);

  useEffect(() => {
    if (!active || !signedIn) return;
    return openAccountStream({
      onAccountChanged: event => handleAccountChangedEvent(queryClient, event),
      onReset: () => {
        void queryClient.invalidateQueries({ queryKey: accountKeys.all });
        void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.all });
      },
    });
  }, [active, queryClient, signedIn]);

  return { signedIn };
}
