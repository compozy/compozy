import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { expectFetchRequest, mockJsonResponse } from "@/test/fetch-test-utils";
import {
  deleteVaultSecret,
  getVaultSecret,
  listVaultSecrets,
  putVaultSecret,
  VaultApiError,
} from "@/systems/vault/adapters/vault-api";

const vaultSecretFixture = {
  created_at: "2026-05-02T10:00:00Z",
  kind: "token",
  namespace: "sessions",
  present: true,
  ref: "vault:sessions/sess_123/github-token",
  updated_at: "2026-05-02T10:00:00Z",
};

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn());
});

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("listVaultSecrets", () => {
  it("calls GET /api/vault/secrets with namespace and prefix filters", async () => {
    mockJsonResponse({ secrets: [vaultSecretFixture] });

    const result = await listVaultSecrets({
      namespace: "sessions",
      prefix: "vault:sessions/sess_123/",
    });

    expect(result).toEqual([vaultSecretFixture]);
    await expectFetchRequest({
      path: "/api/vault/secrets?namespace=sessions&prefix=vault%3Asessions%2Fsess_123%2F",
    });
  });

  it("throws VaultApiError on non-2xx list responses", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 503 }));

    await expect(listVaultSecrets()).rejects.toThrow(VaultApiError);
    await expect(listVaultSecrets()).rejects.toThrow("Failed to list vault secrets: 503");
  });
});

describe("getVaultSecret", () => {
  it("calls GET /api/vault/secrets/metadata and returns redacted metadata", async () => {
    mockJsonResponse({ secret: vaultSecretFixture });

    const result = await getVaultSecret(vaultSecretFixture.ref);

    expect(result).toEqual(vaultSecretFixture);
    await expectFetchRequest({
      path: "/api/vault/secrets/metadata?ref=vault%3Asessions%2Fsess_123%2Fgithub-token",
    });
  });

  it("throws a not found error for unknown refs", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 404 }));

    await expect(getVaultSecret("vault:sessions/sess_123/missing")).rejects.toThrow(
      "Vault secret not found: vault:sessions/sess_123/missing"
    );
  });
});

describe("putVaultSecret", () => {
  it("calls PUT /api/vault/secrets without returning plaintext", async () => {
    mockJsonResponse({ secret: vaultSecretFixture });

    const result = await putVaultSecret({
      ref: vaultSecretFixture.ref,
      kind: "token",
      secret_value: "super-secret-token",
    });

    expect(result).toEqual(vaultSecretFixture);
    expect(JSON.stringify(result)).not.toContain("super-secret-token");
    await expectFetchRequest({
      method: "PUT",
      path: "/api/vault/secrets",
      body: {
        ref: vaultSecretFixture.ref,
        kind: "token",
        secret_value: "super-secret-token",
      },
    });
  });
});

describe("deleteVaultSecret", () => {
  it("calls DELETE /api/vault/secrets with the ref query", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 204 }));

    await deleteVaultSecret(vaultSecretFixture.ref);

    await expectFetchRequest({
      method: "DELETE",
      path: "/api/vault/secrets?ref=vault%3Asessions%2Fsess_123%2Fgithub-token",
    });
  });
});
