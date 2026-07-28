import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type { PutVaultSecretRequest, VaultListFilter, VaultSecret } from "../types";

export class VaultApiError extends Error {
  constructor(
    message: string,
    public readonly status: number
  ) {
    super(message);
    this.name = "VaultApiError";
  }
}

function normalizeOptionalText(value?: string | null): string | undefined {
  if (typeof value !== "string") {
    return undefined;
  }

  const trimmed = value.trim();
  return trimmed === "" ? undefined : trimmed;
}

function normalizeListFilter(filter: VaultListFilter = {}) {
  const normalized: VaultListFilter = {};
  const namespace = normalizeOptionalText(filter.namespace);
  if (namespace) {
    normalized.namespace = namespace;
  }
  const prefix = normalizeOptionalText(filter.prefix);
  if (prefix) {
    normalized.prefix = prefix;
  }
  return normalized;
}

export async function listVaultSecrets(
  filter: VaultListFilter = {},
  signal?: AbortSignal
): Promise<VaultSecret[]> {
  const { data, error, response } = await apiClient.GET("/api/vault/secrets", {
    params: { query: normalizeListFilter(filter) },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    throw new VaultApiError(
      defaultApiErrorMessage("Failed to list vault secrets", response, error),
      response.status
    );
  }

  return requireResponseData(data, response, "Failed to list vault secrets").secrets;
}

export async function getVaultSecret(ref: string, signal?: AbortSignal): Promise<VaultSecret> {
  const { data, error, response } = await apiClient.GET("/api/vault/secrets/metadata", {
    params: { query: { ref } },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new VaultApiError(`Vault secret not found: ${ref}`, 404);
    }

    throw new VaultApiError(
      defaultApiErrorMessage(`Failed to load vault secret "${ref}"`, response, error),
      response.status
    );
  }

  return requireResponseData(data, response, `Failed to load vault secret "${ref}"`).secret;
}

export async function putVaultSecret(
  body: PutVaultSecretRequest,
  signal?: AbortSignal
): Promise<VaultSecret> {
  const { data, error, response } = await apiClient.PUT("/api/vault/secrets", {
    body,
    signal,
  });

  if (apiRequestFailed(response, error)) {
    throw new VaultApiError(
      defaultApiErrorMessage("Failed to store vault secret", response, error),
      response.status
    );
  }

  return requireResponseData(data, response, "Failed to store vault secret").secret;
}

export async function deleteVaultSecret(ref: string, signal?: AbortSignal): Promise<void> {
  const { error, response } = await apiClient.DELETE("/api/vault/secrets", {
    params: { query: { ref } },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new VaultApiError(`Vault secret not found: ${ref}`, 404);
    }

    throw new VaultApiError(
      defaultApiErrorMessage(`Failed to delete vault secret "${ref}"`, response, error),
      response.status
    );
  }
}
