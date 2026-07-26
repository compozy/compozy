// Types
export type {
  PutVaultSecretRequest,
  VaultListFilter,
  VaultNamespace,
  VaultSecret,
  VaultSecretDetail,
  VaultSecretResponse,
  VaultSecretsResponse,
} from "./types";
export { VAULT_NAMESPACES } from "./types";

// Adapters
export {
  deleteVaultSecret,
  getVaultSecret,
  listVaultSecrets,
  putVaultSecret,
  VaultApiError,
} from "./adapters/vault-api";

// Query infrastructure
export { vaultKeys } from "./lib/query-keys";
export { vaultNamespaceTone } from "./lib/vault-tones";
export {
  sessionVaultSecretsOptions,
  vaultSecretDetailOptions,
  vaultSecretsListOptions,
  VAULT_QUERY_INTERVALS,
} from "./lib/query-options";

// Hooks
export { useSessionVaultSecrets, useVaultSecret, useVaultSecrets } from "./hooks/use-vault";
export { useDeleteVaultSecret, usePutVaultSecret } from "./hooks/use-vault-actions";

// Components
export {
  SessionVaultPanel,
  VaultListFilters,
  VaultSecretSheet,
  VaultSecretsCard,
  VaultSecretsList,
  VaultSecretsRow,
  type VaultListFiltersProps,
  type VaultSecretSheetProps,
  type VaultSecretsCardProps,
  type VaultSecretsRowProps,
} from "./components";

export { vaultSecretTitle } from "./lib/vault-secret-title";

export {
  normalizeVaultPrefixForNamespace,
  parseVaultNamespaceFilter,
  useVaultPage,
  validateVaultSearch,
  type VaultDraft,
  type VaultEditorState,
  type VaultLastAction,
  type VaultNamespaceFilter,
  type VaultRouteSearch,
} from "./hooks/use-vault-page";
export { VaultPage } from "./routes/vault-page";
