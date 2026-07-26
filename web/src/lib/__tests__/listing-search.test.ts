import { describe, expect, it } from "vitest";

import { normalizeListingSearchValue, parseListingView } from "../listing-search";

describe("listing-search", () => {
  describe("normalizeListingSearchValue", () => {
    it("Should trim a string value and keep meaningful content", () => {
      expect(normalizeListingSearchValue("query")).toBe("query");
      expect(normalizeListingSearchValue("  query  ")).toBe("query");
    });

    it("Should treat blank and whitespace-only strings as undefined", () => {
      expect(normalizeListingSearchValue("")).toBeUndefined();
      expect(normalizeListingSearchValue("   ")).toBeUndefined();
      expect(normalizeListingSearchValue("\t\n")).toBeUndefined();
    });

    it("Should treat non-string inputs as undefined", () => {
      expect(normalizeListingSearchValue(undefined)).toBeUndefined();
      expect(normalizeListingSearchValue(null)).toBeUndefined();
      expect(normalizeListingSearchValue(42)).toBeUndefined();
      expect(normalizeListingSearchValue(["query"])).toBeUndefined();
      expect(normalizeListingSearchValue({ q: "query" })).toBeUndefined();
    });
  });

  describe("parseListingView", () => {
    it("Should accept the two supported view modes", () => {
      expect(parseListingView("rows")).toBe("rows");
      expect(parseListingView("cards")).toBe("cards");
    });

    it("Should reject unknown or non-string view values", () => {
      expect(parseListingView("grid")).toBeUndefined();
      expect(parseListingView("")).toBeUndefined();
      expect(parseListingView(undefined)).toBeUndefined();
      expect(parseListingView(null)).toBeUndefined();
      expect(parseListingView(0)).toBeUndefined();
      expect(parseListingView(["rows"])).toBeUndefined();
    });
  });
});
