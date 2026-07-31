import { describe, expect, it } from "vitest";

import { marketplaceListings } from "../../mocks";
import {
  formatMarketplaceMCPLaunch,
  formatMarketplaceCount,
  formatMarketplaceVersion,
  isMarketplaceKind,
  isMarketplaceViewSort,
  sortMarketplaceEntries,
} from "../marketplace-ui";

describe("marketplace UI helpers", () => {
  it("Should recognize only route-supported kinds and sort values", () => {
    expect(isMarketplaceKind("skill")).toBe(true);
    expect(isMarketplaceKind("recipe")).toBe(false);
    expect(isMarketplaceViewSort("downloads")).toBe(true);
    expect(isMarketplaceViewSort("updated")).toBe(false);
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
