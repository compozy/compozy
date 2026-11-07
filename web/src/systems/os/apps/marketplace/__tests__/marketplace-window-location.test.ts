// Suite: Marketplace window route parser
// Invariant: a window route names exactly one Marketplace surface — Browse, Installed, an entry's
// detail, one category, or a path the app does not own — with the entry id decoded and each surface's search
// validated at the boundary; the window and its loading fallback read the same table.
import { describe, expect, it } from "vitest";

import { parseMarketplaceWindowLocation } from "../marketplace-window-location";

describe("parseMarketplaceWindowLocation", () => {
  it.each([
    ["/marketplace", "browse"],
    ["/marketplace/installed", "installed"],
    ["/marketplace/compozy-catalog", "detail"],
    ["/marketplace/a/b", "not-found"],
    ["/marketplace/categories/a/b", "not-found"],
    ["/marketplace/categories", "not-found"],
    ["/elsewhere", "not-found"],
  ] as const)("Should read %s as the %s surface", (pathname, kind) => {
    expect(parseMarketplaceWindowLocation({ pathname, search: {} }).kind).toBe(kind);
  });

  // UT-025 [US-002.EC-4]: a category deep link opens the category in a window.
  it("Should read a category path as the category surface and keep detail for one segment [UT-025]", () => {
    expect(
      parseMarketplaceWindowLocation({
        pathname: "/marketplace/categories/design",
        search: { q: " fig " },
      })
    ).toEqual({ kind: "category", category: "design", search: { q: "fig" } });
    expect(
      parseMarketplaceWindowLocation({ pathname: "/marketplace/x", search: {} })
    ).toMatchObject({ kind: "detail", entryId: "x" });
  });

  it("Should decode the entry id and keep a malformed escape as written", () => {
    expect(
      parseMarketplaceWindowLocation({ pathname: "/marketplace/skills%2Fplanner", search: {} })
    ).toMatchObject({ kind: "detail", entryId: "skills/planner" });
    expect(
      parseMarketplaceWindowLocation({ pathname: "/marketplace/%E0%A4%A", search: {} })
    ).toMatchObject({ kind: "detail", entryId: "%E0%A4%A" });
  });

  it("Should validate the search of the surface it names", () => {
    expect(
      parseMarketplaceWindowLocation({ pathname: "/marketplace", search: { q: "  planner " } })
    ).toMatchObject({ kind: "browse", search: { q: "planner" } });
    expect(
      parseMarketplaceWindowLocation({
        pathname: "/marketplace/entry-1",
        search: { from: "installed", scope: "bogus", q: "x", setup: "1" },
      })
    ).toMatchObject({
      kind: "detail",
      search: { from: "installed", scope: undefined, q: "x", setup: 1 },
    });
  });
});
