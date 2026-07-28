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

export function useVaultSecrets(filter: VaultListFilter = {}) {
  return useQuery(vaultSecretsListOptions(filter));
}

export function useVaultSecret(ref: string, options: QueryEnabledOptions = {}) {
  return useQuery(vaultSecretDetailOptions(ref, options.enabled ?? true));
}

export function useSessionVaultSecrets(sessionId: string, options: QueryEnabledOptions = {}) {
  return useQuery(sessionVaultSecretsOptions(sessionId, options.enabled ?? true));
}
