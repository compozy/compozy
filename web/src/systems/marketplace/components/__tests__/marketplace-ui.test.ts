// Invariant: the marketplace formats versions consistently and strips retired query controls.
// Owner: marketplace presentation helpers; canonical suite: marketplace-ui.test.ts.
import { describe, expect, it } from "vitest";
import { formatMarketplaceVersion } from "../marketplace-ui";
import { validateMarketplaceSearch } from "../../lib/marketplace-search";

describe("marketplace UI helpers", () => {
  it.each([
    ["1.8.0", "v1.8.0"],
    ["v1.8.0", "v1.8.0"],
    ["V1.8.0", "v1.8.0"],
    ["Vv1.8.0", "v1.8.0"],
  ])("Should render marketplace version %s with exactly one prefix", (version, expected) => {
    expect(formatMarketplaceVersion(version)).toBe(expected);
  });

  it("Should retain only the normalized search query", () => {
    expect(validateMarketplaceSearch({ q: "  audit  ", tab: "installed", kind: "mcp" })).toEqual({
      q: "audit",
    });
    expect(validateMarketplaceSearch({ q: ["audit"] })).toEqual({ q: undefined });
  });
});
