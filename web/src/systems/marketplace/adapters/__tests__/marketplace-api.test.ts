import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { expectFetchRequest, mockJsonResponse } from "@/test/fetch-test-utils";
import {
  browseMarketplace,
  getMarketplaceCatalogEntry,
} from "@/systems/marketplace/adapters/marketplace-api";
import {
  updateMarketplaceExtensions,
  installMarketplaceExtension,
  refreshMarketplaceCatalog,
} from "@/systems/marketplace/adapters/marketplace-actions-api";

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn());
});

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("marketplace acquisition transport", () => {
  it("Should refresh the unified feed catalog", async () => {
    mockJsonResponse({
      kinds: [{ entry_count: 4, kind: "extension", outcome: "refreshed", stale: false }],
    });

    await refreshMarketplaceCatalog();

    await expectFetchRequest({
      method: "POST",
      path: "/api/marketplace/refresh?kind=extension",
    });
  });

  it("Should install extensions through the source-union mutation", async () => {
    const extensionBody = {
      allow_unverified: true,
      ref: "review-pack",
      source: "curated",
      version: "2.0.0",
    };
    mockJsonResponse({ extension: { name: "review-pack" } }, { status: 201 });
    await installMarketplaceExtension(extensionBody);
    await expectFetchRequest({
      body: extensionBody,
      method: "POST",
      path: "/api/extensions",
    });
  });

  it.each([
    ["refresh", () => refreshMarketplaceCatalog()],
    [
      "extension install",
      () => installMarketplaceExtension({ ref: "review-pack", source: "curated" }),
    ],
  ])("Should preserve typed daemon errors for %s failures", async (_label, request) => {
    mockJsonResponse({ error: "catalog mutation rejected" }, { status: 409 });

    await expect(request()).rejects.toMatchObject({
      message: "catalog mutation rejected",
      status: 409,
    });
  });

  it("Should preserve the daemon diagnostic code for extension consent decisions", async () => {
    mockJsonResponse(
      {
        diagnostic: { code: "extension_checksum_unverified" },
        error: "extension checksum is not registry-verified",
      },
      { status: 422 }
    );

    await expect(
      installMarketplaceExtension({ ref: "/srv/hello", source: "local_path" })
    ).rejects.toMatchObject({
      diagnosticCode: "extension_checksum_unverified",
      status: 422,
    });
  });

  // Invariant: install consent and input remediation retain structured daemon evidence at the HTTP boundary.
  it("Should send the approved digest and inputs and preserve a source-changed refusal", async () => {
    const listedDigest = "a".repeat(64);
    const fetchedDigest = "b".repeat(64);
    const body = {
      source: "curated" as const,
      ref: "compozy/sentry",
      expected_digest: listedDigest,
      inputs: { token: { vault_ref: "vault:mcp/shared/TOKEN" }, region: { value: "eu" } },
    };
    mockJsonResponse(
      {
        code: "extension_source_changed",
        error: "source changed",
        listed_digest: listedDigest,
        fetched_digest: fetchedDigest,
      },
      { status: 409 }
    );
    await expect(installMarketplaceExtension(body)).rejects.toMatchObject({
      status: 409,
      diagnosticCode: "extension_source_changed",
      listedDigest,
      fetchedDigest,
    });
    await expectFetchRequest({ method: "POST", path: "/api/extensions", body });
  });

  it("Should retain an invalid input id without treating a malformed input list as configuration", async () => {
    mockJsonResponse(
      { code: "extension_input_invalid", error: "invalid input", input_id: "region", inputs: [42] },
      { status: 422 }
    );
    await expect(
      installMarketplaceExtension({ source: "curated", ref: "compozy/sentry" })
    ).rejects.toMatchObject({ inputId: "region", requiredInputs: undefined, status: 422 });
  });
});

// Invariant: one-catalog transport preserves envelope, scoped identity, cancellation and cursor restart errors.
// Owning layer: HTTP adapter; canonical suite: marketplace-api.test.ts.
describe("one catalog transport", () => {
  it("Should preserve a degraded envelope and send the source-independent scoped cursor", async () => {
    const envelope = {
      total: 0,
      revision: "revision-a",
      stale: true,
      sources: [{ name: "compozy-catalog", kind: "feed", state: "degraded", count: 0 }],
      items: [],
    };
    mockJsonResponse(envelope);
    const controller = new AbortController();
    const signal = controller.signal;
    await expect(
      browseMarketplace(
        {
          q: " audit ",
          cursor: " cursor-a ",
          limit: 50,
          workspaceId: " ws-a ",
          profileName: " work ",
        },
        signal
      )
    ).resolves.toEqual(envelope);
    await expectFetchRequest({
      path: "/api/marketplace?q=audit&limit=50&cursor=cursor-a&profile=work&scope=workspace&workspace_id=ws-a",
    });
    const request = vi.mocked(fetch).mock.calls[0]?.[0] as Request;
    controller.abort();
    expect(request.signal.aborted).toBe(true);
  });

  it("Should preserve a typed restart instruction without treating other conflicts as cursor resets", async () => {
    mockJsonResponse(
      { error: "Catalog changed", code: "marketplace_cursor_stale", restart: true },
      { status: 409 }
    );
    await expect(browseMarketplace({ cursor: "old" })).rejects.toMatchObject({
      status: 409,
      diagnosticCode: "marketplace_cursor_stale",
      restart: true,
    });
  });

  it("Should qualify detail by source and installed name inside the selected profile", async () => {
    const response = { entry: { entry_id: "entry-a", installed: true } };
    mockJsonResponse(response);
    await expect(
      getMarketplaceCatalogEntry({
        entryId: " entry-a ",
        source: " compozy-catalog ",
        installedName: " local-name ",
        profileName: " work ",
        workspaceId: " ws-a ",
      })
    ).resolves.toEqual(response);
    await expectFetchRequest({
      path: "/api/marketplace/entries/entry-a?source=compozy-catalog&installed_name=local-name&profile=work&scope=workspace&workspace_id=ws-a",
    });
  });
});

// Invariant: batch update results retain individual failures instead of claiming atomic success.
// Owning layer: extension update transport; canonical suite: marketplace-api.test.ts (IT-007 wire coverage).
describe("marketplace batch update transport", () => {
  it("Should send the exact names once and preserve mixed server outcomes", async () => {
    const body = { names: ["first", "second"] };
    const response = {
      updates: [
        { name: "first", status: "updated", latest_version: "2.0.0" },
        { name: "second", status: "failed", error: { message: "Artifact unavailable" } },
      ],
    };
    mockJsonResponse(response);
    await expect(updateMarketplaceExtensions(body)).resolves.toEqual(response);
    await expectFetchRequest({ path: "/api/extensions/update", method: "POST", body });
  });
});
