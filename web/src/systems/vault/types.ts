import type { OperationQuery, OperationRequestBody, OperationResponse } from "@/lib/api-contract";

export type VaultSecretsResponse = OperationResponse<"listVaultSecrets", 200>;
export type VaultSecret = VaultSecretsResponse["secrets"][number];
export type VaultSecretResponse = OperationResponse<"getVaultSecretMetadata", 200>;
export type VaultSecretDetail = VaultSecretResponse["secret"];
export type PutVaultSecretRequest = OperationRequestBody<"putVaultSecret">;
export type VaultListFilter = NonNullable<OperationQuery<"listVaultSecrets">>;

export const VAULT_NAMESPACES = [
  "automation",
  "bridges",
  "extensions",
  "hooks",
  "mcp",
  "providers",
  "sandbox",
  "sessions",
] as const;

export type VaultNamespace = (typeof VAULT_NAMESPACES)[number];
