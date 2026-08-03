import { describe, expect, it } from "vitest";

import { marketplaceListings } from "../../mocks";
import { marketplaceKindConfig } from "../../lib/marketplace-kind-config";
import {
  MARKETPLACE_KINDS,
  MARKETPLACE_ROUTE_KINDS,
  marketplaceApiKindFor,
  marketplaceRouteKindFor,
} from "../../types";
import {
  formatMarketplaceMCPLaunch,
  formatMarketplaceCount,
  formatMarketplaceVersion,
  isMarketplaceKind,
  isMarketplaceViewSort,
  MARKETPLACE_KIND_LABEL,
  MARKETPLACE_KIND_ORDER,
  MARKETPLACE_KIND_SINGULAR,
  sortMarketplaceEntries,
} from "../marketplace-ui";

describe("marketplace UI helpers", () => {
  it("Should recognize only route-supported kinds and sort values", () => {
    expect(isMarketplaceKind("skill")).toBe(true);
    expect(isMarketplaceKind("recipe")).toBe(false);
    expect(isMarketplaceViewSort("downloads")).toBe(true);
    expect(isMarketplaceViewSort("updated")).toBe(false);
  });

  // UT-062: the kind system is the marketplace's public surface — every table that keys off it
  // must agree, so a re-introduced kind cannot reach a page through one of them.
  it("Should expose exactly extension, skill, and mcp across every kind table", () => {
    expect([...MARKETPLACE_KINDS].sort()).toEqual(["extension", "mcp", "skill"]);
    expect([...MARKETPLACE_ROUTE_KINDS].sort()).toEqual(["extensions", "mcps", "skills"]);
    expect([...MARKETPLACE_KIND_ORDER].sort()).toEqual([...MARKETPLACE_KINDS].sort());
    expect(Object.keys(MARKETPLACE_KIND_LABEL).sort()).toEqual([...MARKETPLACE_KINDS].sort());
    expect(Object.keys(MARKETPLACE_KIND_SINGULAR).sort()).toEqual([...MARKETPLACE_KINDS].sort());
    expect(isMarketplaceKind("bundle")).toBe(false);

    for (const kind of MARKETPLACE_KINDS) {
      expect(marketplaceKindConfig(kind).kind).toBe(kind);
      expect(marketplaceApiKindFor(marketplaceRouteKindFor(kind))).toBe(kind);
    }
  });

  it("Should sort by real downloads or name without mutating the API order", () => {
    const source = marketplaceListings.skill.slice(0, 3);

    expect(sortMarketplaceEntries(source, "downloads").map(entry => entry.name)).toEqual([
      "git-flow",
      "qa-bootstrap",
      "docs-sync",
    ]);
    expect(sortMarketplaceEntries(source, "name").map(entry => entry.name)).toEqual([
      "docs-sync",
      "git-flow",
      "qa-bootstrap",
    ]);
    expect(source.map(entry => entry.name)).toEqual(["git-flow", "docs-sync", "qa-bootstrap"]);
  });

  it("Should keep compact count formatting deterministic", () => {
    expect(formatMarketplaceCount(840)).toBe("840");
    expect(formatMarketplaceCount(3400)).toBe("3.4K");
  });

  it.each([
    ["1.8.0", "v1.8.0"],
    ["v1.8.0", "v1.8.0"],
    ["V1.8.0", "v1.8.0"],
    ["Vv1.8.0", "v1.8.0"],
  ])("Should render marketplace version %s with exactly one prefix", (version, expected) => {
    expect(formatMarketplaceVersion(version)).toBe(expected);
  });

  it("Should format every MCP launch distribution through one catalog formatter", () => {
    expect(formatMarketplaceMCPLaunch({ type: "remote", url: "https://mcp.linear.app/mcp" })).toBe(
      "https://mcp.linear.app/mcp"
    );
    expect(
      formatMarketplaceMCPLaunch({
        args: ["--read-only"],
        digest: "sha256:abc",
        image: "ghcr.io/github/github-mcp-server",
        type: "docker",
      })
    ).toBe("ghcr.io/github/github-mcp-server sha256:abc --read-only");
    expect(
      formatMarketplaceMCPLaunch({
        args: ["--stdio"],
        package: "@modelcontextprotocol/server-github",
        type: "npx",
        version: "1.2.3",
      })
    ).toBe("npx @modelcontextprotocol/server-github 1.2.3 --stdio");
  });
});
