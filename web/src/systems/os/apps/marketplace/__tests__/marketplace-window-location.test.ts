// Suite: Marketplace window route parser
// Invariant: a window route names exactly one Marketplace surface — Browse, Installed, an entry's
// detail, or a path the app does not own — with the entry id decoded and each surface's search
// validated at the boundary; the window and its loading fallback read the same table.
import { describe, expect, it } from "vitest";

import { parseMarketplaceWindowLocation } from "../marketplace-window-location";

describe("parseMarketplaceWindowLocation", () => {
  it.each([
    ["/marketplace", "browse"],
    ["/marketplace/installed", "installed"],
    ["/marketplace/compozy-catalog", "detail"],
    ["/marketplace/a/b", "not-found"],
    ["/elsewhere", "not-found"],
  ] as const)("Should read %s as the %s surface", (pathname, kind) => {
    expect(parseMarketplaceWindowLocation({ pathname, search: {} }).kind).toBe(kind);
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
        search: { from: "installed", scope: "bogus", q: "x" },
      })
    ).toMatchObject({ kind: "detail", search: { from: "installed", scope: undefined, q: "x" } });
  });
});
