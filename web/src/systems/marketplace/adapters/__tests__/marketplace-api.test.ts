import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { expectFetchRequest, mockJsonResponse } from "@/test/fetch-test-utils";
import {
  browseMarketplaceKind,
  getMarketplaceEntry,
  MarketplaceApiError,
  searchMarketplace,
} from "@/systems/marketplace/adapters/marketplace-api";
import {
  activateMarketplaceBundle,
  installMarketplaceExtension,
  installMarketplaceMCP,
  installMarketplaceSkill,
  previewMarketplaceBundle,
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

  it("Should omit installed identity for bundle detail", async () => {
    mockJsonResponse({
      entry: {
        description: "Review agent work",
        entry_id: "review/strict",
        installed: false,
        kind: "bundle",
        name: "Strict review",
        source: "extension",
        update_available: false,
      },
    });

    await getMarketplaceEntry({
      entryId: " review/strict ",
      kind: "bundle",
      workspaceId: null,
    });

    await expectFetchRequest({
      path: "/api/marketplace/bundle/review%2Fstrict?scope=global",
    });
  });
});

describe("marketplace acquisition transport", () => {
  it("Should install an MCP entry through the generated catalog endpoint", async () => {
    const body = {
      entry_id: "github-mcp",
      scope: "workspace" as const,
      values: { env: { GITHUB_TOKEN: { vault_ref: "vault:mcp/github/token" } } },
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

  it("Should preview a bundle without activating it", async () => {
    const body = {
      bundle_name: "review-team",
      extension_name: "review-pack",
      profile_name: "strict",
      scope: "workspace",
      workspace: "ws-a",
    };
    mockJsonResponse({ activation: { id: "preview" } });

    await previewMarketplaceBundle(body);

    await expectFetchRequest({ body, method: "POST", path: "/api/bundles/preview" });
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
    const installBody = { slug: "@agh/reviewer", version: "1.2.0" };
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

  it("Should install extensions and activate bundles through separate mutations", async () => {
    const extensionBody = {
      allow_unverified: true,
      slug: "review-pack",
      version: "2.0.0",
    };
    mockJsonResponse({ extension: { name: "review-pack" } }, { status: 201 });
    await installMarketplaceExtension(extensionBody);
    await expectFetchRequest({
      body: extensionBody,
      method: "POST",
      path: "/api/extensions",
    });

    const activationBody = {
      bundle_name: "review-team",
      confirm_network_requirement: true,
      extension_name: "review-pack",
      profile_name: "strict",
      scope: "global",
    };
    mockJsonResponse({ activation: { id: "activation-1" } }, { status: 201 });
    await activateMarketplaceBundle(activationBody);
    await expectFetchRequest({
      body: activationBody,
      callIndex: 1,
      method: "POST",
      path: "/api/bundles/activations",
    });
  });

  it.each([
    ["refresh", () => refreshMarketplaceCatalog()],
    [
      "MCP install",
      () => installMarketplaceMCP({ entry_id: "github", scope: "global", values: null }),
    ],
    ["skill install", () => installMarketplaceSkill({ slug: "@agh/reviewer" })],
    ["skill update", () => updateMarketplaceSkill({ name: "reviewer" })],
    ["extension install", () => installMarketplaceExtension({ slug: "review-pack" })],
    [
      "bundle preview",
      () =>
        previewMarketplaceBundle({
          bundle_name: "review-team",
          extension_name: "review-pack",
          profile_name: "strict",
          scope: "global",
        }),
    ],
    [
      "bundle activation",
      () =>
        activateMarketplaceBundle({
          bundle_name: "review-team",
          confirm_network_requirement: true,
          extension_name: "review-pack",
          profile_name: "strict",
          scope: "global",
        }),
    ],
  ])("Should preserve typed daemon errors for %s failures", async (_label, request) => {
    mockJsonResponse({ error: "catalog mutation rejected" }, { status: 409 });

    await expect(request()).rejects.toMatchObject({
      message: "catalog mutation rejected",
      status: 409,
    });
  });
});
