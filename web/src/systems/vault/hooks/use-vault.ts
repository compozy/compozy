import { useQuery } from "@tanstack/react-query";

import {
  sessionVaultSecretsOptions,
  vaultSecretDetailOptions,
  vaultSecretsListOptions,
} from "../lib/query-options";
import type { VaultListFilter } from "../types";

interface QueryEnabledOptions {
  enabled?: boolean;
}

export function useVaultSecrets(filter: VaultListFilter = {}, options: QueryEnabledOptions = {}) {
  return useQuery({
    ...vaultSecretsListOptions(filter),
    enabled: options.enabled ?? true,
  });
}

export function useVaultSecret(ref: string, options: QueryEnabledOptions = {}) {
  return useQuery(vaultSecretDetailOptions(ref, options.enabled ?? true));
}

export function useSessionVaultSecrets(sessionId: string, options: QueryEnabledOptions = {}) {
  return useQuery(sessionVaultSecretsOptions(sessionId, options.enabled ?? true));
}
