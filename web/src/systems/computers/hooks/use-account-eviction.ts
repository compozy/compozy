import { useEffect, useRef } from "react";
import { useQuery, useQueryClient, type QueryClient } from "@tanstack/react-query";

import { evictAccountScopedState } from "@/lib/computer-scope";
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
  const email = data.email?.trim();
  if (email) return email.toLowerCase();
  const deviceId = data.device?.id?.trim();
  if (deviceId) return deviceId;
  return data.source ?? "signed_in";
}

function handleAccountChangedEvent(queryClient: QueryClient, event: AccountChangedEventData): void {
  if (event.evict_remote) {
    // Cancel and remove all prior account- and computer-scoped caches and stores so none of the
    // previous account's metadata survives while the new account status loads (Rule 23, AST-010).
    evictAccountScopedState(queryClient);
    return;
  }
  void queryClient.invalidateQueries({ queryKey: accountKeys.all });
  void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.all });
}

/**
 * Evicts all account-scoped (`["account", ...]`), computer-registry (`["computers", ...]`), and
 * remote-computer (`["computer", id, ...]`) caches and stores whenever the signed-in account signs
 * out or switches (Rule 23, US-001.EC-4, IT-026, AST-010).
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
    // `accountQuery.data` already holds the newly confirmed status for `accountIdentity`;
    // cancel and remove every other account- and computer-scoped cache and store.
    evictAccountScopedState(queryClient, { preserveAccountStatus: true });
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
