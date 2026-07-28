import { describe, expect, it } from "vitest";

import { marketplaceListings } from "../../mocks";
import {
  formatMarketplaceCount,
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
});
