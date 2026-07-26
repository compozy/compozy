import { HttpResponse, type HttpHandler } from "msw";

import { aghApiMock } from "@/storybook/openapi-msw";

import { vaultSecretFixtures, vaultSecretsResponseFixture } from "./fixtures";

function filterVaultSecrets(request: Request) {
  const url = new URL(request.url);
  const namespace = url.searchParams.get("namespace");
  const prefix = url.searchParams.get("prefix");

  return vaultSecretFixtures.filter(secret => {
    if (namespace && secret.namespace !== namespace) return false;
    if (prefix && !secret.ref.startsWith(prefix)) return false;
    return true;
  });
}

function vaultNamespaceFor(ref: string) {
  const [, rest = ""] = ref.split(":");
  return rest.split("/")[0] || "sessions";
}

function readRecord(value: unknown): Record<string, unknown> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    return {};
  }

  return value as Record<string, unknown>;
}

export const handlers: HttpHandler[] = [
  aghApiMock.get("/api/vault/secrets", ({ request }) =>
    HttpResponse.json({ secrets: filterVaultSecrets(request) })
  ),
  aghApiMock.get("/api/vault/secrets/metadata", ({ request }) => {
    const url = new URL(request.url);
    const ref = url.searchParams.get("ref") ?? "";
    const secret = vaultSecretFixtures.find(candidate => candidate.ref === ref);

    if (!secret) {
      return HttpResponse.json({ error: `Vault secret not found: ${ref}` }, { status: 404 });
    }

    return HttpResponse.json({ secret });
  }),
  aghApiMock.put("/api/vault/secrets", async ({ request }) => {
    const body = readRecord(await request.json());
    const ref = typeof body.ref === "string" ? body.ref : "vault:sessions/storybook";
    return HttpResponse.json({
      secret: {
        ref,
        namespace: vaultNamespaceFor(ref),
        kind: typeof body.kind === "string" ? body.kind : "api_key",
        present: true,
        created_at: "2026-04-17T12:00:00Z",
        updated_at: "2026-04-17T12:00:00Z",
      },
    });
  }),
  aghApiMock.delete("/api/vault/secrets", ({ request }) => {
    const url = new URL(request.url);
    const ref = url.searchParams.get("ref") ?? "";
    const secret = vaultSecretsResponseFixture.secrets.find(candidate => candidate.ref === ref);

    if (!secret) {
      return HttpResponse.json({ error: `Vault secret not found: ${ref}` }, { status: 404 });
    }

    return new HttpResponse(null, { status: 204 });
  }),
];
