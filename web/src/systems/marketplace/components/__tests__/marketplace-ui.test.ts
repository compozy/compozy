// Invariant: the marketplace formats counts and versions consistently and strips retired query controls.
// Owner: marketplace presentation helpers; canonical suite: marketplace-ui.test.ts.
import { describe, expect, it } from "vitest";
import { formatMarketplaceCount, formatMarketplaceVersion } from "../marketplace-ui";
import { validateMarketplaceSearch } from "../../lib/marketplace-search";

describe("marketplace UI helpers", () => {
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

  it("Should retain only the normalized search query", () => {
    expect(validateMarketplaceSearch({ q: "  audit  ", tab: "installed", kind: "mcp" })).toEqual({
      q: "audit",
    });
    expect(validateMarketplaceSearch({ q: ["audit"] })).toEqual({ q: undefined });
  });
});
