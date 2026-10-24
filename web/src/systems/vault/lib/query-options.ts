import { queryOptions } from "@tanstack/react-query";

import { getVaultSecret, listVaultSecrets } from "../adapters/vault-api";
import type { VaultListFilter } from "../types";
import { vaultKeys } from "./query-keys";

const VAULT_STALE_TIME = 15_000;
const VAULT_REFETCH_INTERVAL = 45_000;

/** The namespace a prefix edit leaves alone: the key without its trailing prefix. */
function vaultListNamespace(queryKey: readonly unknown[]): string {
  return JSON.stringify(queryKey.slice(0, -1));
}

export function vaultSecretsListOptions(filter: VaultListFilter = {}) {
  const queryKey = vaultKeys.list(filter);
  return queryOptions({
    queryKey,
    queryFn: ({ signal }) => listVaultSecrets(filter, signal),
    // Typing a prefix keeps the previous rows on screen; another namespace shows nothing stale.
    placeholderData: (previousData, previousQuery) =>
      previousQuery && vaultListNamespace(previousQuery.queryKey) === vaultListNamespace(queryKey)
        ? previousData
        : undefined,
    staleTime: VAULT_STALE_TIME,
    refetchInterval: VAULT_REFETCH_INTERVAL,
  });
}

export function vaultSecretDetailOptions(ref: string, enabled = true) {
  return queryOptions({
    queryKey: vaultKeys.detail(ref),
    queryFn: ({ signal }) => getVaultSecret(ref, signal),
    enabled: Boolean(ref.trim()) && enabled,
    staleTime: VAULT_STALE_TIME,
    refetchInterval: VAULT_REFETCH_INTERVAL,
  });
}

export function sessionVaultSecretsOptions(sessionId: string, enabled = true) {
  const prefix = sessionId.trim() ? `vault:sessions/${sessionId.trim()}/` : "";
  return queryOptions({
    queryKey: vaultKeys.session(sessionId),
    queryFn: ({ signal }) => listVaultSecrets({ namespace: "sessions", prefix }, signal),
    enabled: Boolean(sessionId.trim()) && enabled,
    staleTime: VAULT_STALE_TIME,
    refetchInterval: VAULT_REFETCH_INTERVAL,
  });
}

export const VAULT_QUERY_INTERVALS = {
  staleTime: VAULT_STALE_TIME,
  refetchInterval: VAULT_REFETCH_INTERVAL,
} as const;
