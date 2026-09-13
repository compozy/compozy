import { describe, expect, it } from "vitest";
import { marketplaceKeys } from "../query-keys";

// Invariant: distinct source/profile/workspace identities cannot share catalog detail or listing caches.
// Owning layer: query key identity; canonical suite: query-keys.test.ts.
describe("one catalog cache identity", () => {
  it("Should isolate global and workspace discovery and installed identities", () => {
    expect(marketplaceKeys.catalog({ workspaceId: null })).not.toEqual(
      marketplaceKeys.catalog({ workspaceId: "ws-a" })
    );
    expect(marketplaceKeys.catalog({ workspaceId: "ws-a" })).not.toEqual(
      marketplaceKeys.catalog({ workspaceId: "ws-b" })
    );
    expect(marketplaceKeys.catalogEntry({ entryId: "same", installedName: "one" })).not.toEqual(
      marketplaceKeys.catalogEntry({ entryId: "same", installedName: "two" })
    );
    expect(
      marketplaceKeys.catalogEntry({
        entryId: " same ",
        installedName: " local ",
        source: " feed ",
        workspaceId: " ws-a ",
        profileName: " work ",
      })
    ).toEqual(
      marketplaceKeys.catalogEntry({
        entryId: "same",
        installedName: "local",
        source: "feed",
        workspaceId: "ws-a",
        profileName: "work",
      })
    );
  });

  it("Should isolate profile and source while normalizing equivalent options", () => {
    expect(
      marketplaceKeys.catalog({ q: " audit ", profileName: " work ", workspaceId: " ws-a " })
    ).toEqual(
      marketplaceKeys.catalog({ q: "audit", profileName: "work", workspaceId: "ws-a", limit: 100 })
    );
    expect(marketplaceKeys.catalog({ profileName: "work" })).not.toEqual(
      marketplaceKeys.catalog({ profileName: "personal" })
    );
    expect(marketplaceKeys.catalogEntry({ source: "a", entryId: "same" })).not.toEqual(
      marketplaceKeys.catalogEntry({ source: "b", entryId: "same" })
    );
    expect(marketplaceKeys.catalogEntry({ entryId: "same", workspaceId: "a" })).not.toEqual(
      marketplaceKeys.catalogEntry({ entryId: "same", workspaceId: "b" })
    );
  });
});
