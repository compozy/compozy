import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { expectFetchRequest, mockJsonResponse } from "@/test/fetch-test-utils";
import {
  browseMarketplace,
  getMarketplaceCatalogEntry,
  browseMarketplaceKind,
  getMarketplaceEntry,
  MarketplaceApiError,
  searchMarketplace,
} from "@/systems/marketplace/adapters/marketplace-api";
import {
  updateMarketplaceExtensions,
  installMarketplaceExtension,
  installMarketplaceMCP,
  installMarketplaceSkill,
  refreshMarketplaceCatalog,
  updateMarketplaceSkill,
} from "@/systems/marketplace/adapters/marketplace-actions-api";

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn());
});

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("searchMarketplace", () => {
  it("Should preserve per-kind errors from one workspace-scoped fan-out request", async () => {
    const response = {
      query: "audit",
      kinds: [
        {
          kind: "extension",
          error: "extension catalog unavailable",
          items: [],
        },
        {
          kind: "skill",
          items: [
            {
              kind: "skill",
              entry_id: "audit-skill",
              name: "Audit skill",
              description: "Audit agent work",
              source: "registry",
              installed: false,
              update_available: false,
            },
          ],
        },
      ],
    };
    mockJsonResponse(response);

    await expect(
      searchMarketplace({ limit: 12, q: "audit", workspaceId: "ws-a" })
    ).resolves.toEqual(response);
    await expectFetchRequest({
      path: "/api/marketplace/search?q=audit&limit=12&scope=workspace&workspace_id=ws-a",
    });
  });

  it("Should throw a typed error for a failed grouped request", async () => {
    mockJsonResponse({ error: "marketplace unavailable" }, { status: 503 });

    const request = searchMarketplace({ q: "audit", workspaceId: null });
    await expect(request).rejects.toBeInstanceOf(MarketplaceApiError);
    await expect(request).rejects.toMatchObject({
      message: "marketplace unavailable",
      status: 503,
    });
  });
});

describe("marketplace browse transport", () => {
  it("Should browse one kind with normalized workspace scope", async () => {
    mockJsonResponse({ items: [], kind: "skill", total: 0 });

    await browseMarketplaceKind({
      kind: "skill",
      limit: 24,
      q: " review ",
      cursor: " next-page ",
      workspaceId: " ws-a ",
    });

    await expectFetchRequest({
      path: "/api/marketplace/skill?q=review&limit=24&scope=workspace&workspace_id=ws-a&cursor=next-page",
    });
  });

  it("Should load installed detail by the stable feed entry id and supported identity", async () => {
    mockJsonResponse({
      entry: {
        description: "Review agent work",
        entry_id: "review/strict",
        installed: false,
        kind: "skill",
        name: "Strict review",
        source: "registry",
        update_available: false,
      },
    });

    await getMarketplaceEntry({
      entryId: " review/strict ",
      installedName: " local-review ",
      kind: "skill",
      workspaceId: null,
    });

    await expectFetchRequest({
      path: "/api/marketplace/skill/review%2Fstrict?scope=global&installed_name=local-review",
    });
  });
});

describe("marketplace acquisition transport", () => {
  it("Should install an MCP entry through the generated catalog endpoint", async () => {
    const body = {
      entry_id: "github-mcp",
      scope: "workspace" as const,
      values: {
        inputs: {
          github_personal_access_token: { vault_ref: "vault:mcp/github/token" },
        },
      },
      workspace_id: "ws-a",
    };
    mockJsonResponse({
      mcp_server: {
        name: "github",
        scope: "workspace",
        source_metadata: {
          available_targets: [],
          effective_source: {
            kind: "workspace-config",
            scope: "workspace",
            workspace_id: "ws-a",
          },
          shadowed_sources: [],
        },
        transport: "stdio",
        workspace_id: "ws-a",
      },
      next_step: "authorize",
    });

    await installMarketplaceMCP(body);

    await expectFetchRequest({
      body,
      method: "POST",
      path: "/api/settings/mcp-servers/install",
    });
  });

  it("Should refresh only a feed-backed kind", async () => {
    mockJsonResponse({
      kinds: [{ entry_count: 4, kind: "extension", outcome: "refreshed", stale: false }],
    });

    await refreshMarketplaceCatalog("extension");

    await expectFetchRequest({
      method: "POST",
      path: "/api/marketplace/refresh?kind=extension",
    });
  });

  it("Should install and update skills through their marketplace endpoints", async () => {
    const installBody = { slug: "@compozy/reviewer", version: "1.2.0" };
    mockJsonResponse({ skill: { name: "reviewer" } });
    await installMarketplaceSkill(installBody);
    await expectFetchRequest({
      body: installBody,
      method: "POST",
      path: "/api/skills/marketplace/install",
    });

    const updateBody = { name: "reviewer" };
    mockJsonResponse({ skills: [] });
    await updateMarketplaceSkill(updateBody);
    await expectFetchRequest({
      body: updateBody,
      callIndex: 1,
      method: "POST",
      path: "/api/skills/marketplace/update",
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
      "MCP install",
      () => installMarketplaceMCP({ entry_id: "github", scope: "user", values: null }),
    ],
    ["skill install", () => installMarketplaceSkill({ slug: "@compozy/reviewer" })],
    ["skill update", () => updateMarketplaceSkill({ name: "reviewer" })],
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
