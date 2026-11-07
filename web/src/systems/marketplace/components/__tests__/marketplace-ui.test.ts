// Invariant: the marketplace formats versions, trust, source names and row setup state consistently.
// Owner: marketplace presentation helpers; canonical suite: marketplace-ui.test.ts.
import { describe, expect, it } from "vitest";
import type { InstalledExtensionView } from "@/systems/extensions";
import { extensionFixtures } from "@/systems/extensions/mocks";
import { marketplaceSetupState } from "../../lib/marketplace-setup-state";
import { marketplaceSourceLabel } from "../../lib/marketplace-installed-view";
import { formatMarketplaceVersion, marketplaceTrustSentence } from "../marketplace-ui";
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

  it.each([
    [{ tier: "official", trust: { checksum_verified: true } }, "Official · checked by CompozyOS"],
    [{ tier: "community", trust: { checksum_verified: false } }, "Community · not verified"],
    [{ tier: undefined, trust: null }, "Community · not verified"],
  ])("Should summarize trust in one plain sentence", (entry, expected) => {
    expect(marketplaceTrustSentence(entry)).toBe(expected);
  });

  it("Should retain only the normalized search query", () => {
    expect(validateMarketplaceSearch({ q: "  audit  ", tab: "installed", kind: "mcp" })).toEqual({
      q: "audit",
    });
    expect(validateMarketplaceSearch({ q: ["audit"] })).toEqual({ q: undefined });
  });

  describe("row setup state", () => {
    const entry = { installed: true, installed_name: "notion" };
    const server = extensionFixtures[0]!.mcp_servers?.[0];
    function item(overrides: Partial<InstalledExtensionView["extension"]>): InstalledExtensionView {
      return {
        extension: {
          ...extensionFixtures[0]!,
          name: "notion",
          enabled: true,
          missing_inputs: [],
          mcp_servers: [],
          inputs: [],
          ...overrides,
        },
        listing: null,
        updateAvailable: false,
      };
    }
    const authServer = {
      ...(server ?? { name: "notion", owner: "extension:notion", scope: "global" }),
      name: "notion",
      status: "needs_authorization",
    } as InstalledExtensionView["extension"]["mcp_servers"][number];

    // UT-031 [US-006, US-011.EC-3]: Set up comes from daemon inventory truth, never the listing.
    it.each<[string, Partial<InstalledExtensionView["extension"]>, string]>([
      ["a missing required input", { missing_inputs: ["LINEAR_API_KEY"] }, "setup"],
      ["a server waiting for sign-in", { mcp_servers: [authServer] }, "setup"],
      [
        "inputs and sign-in satisfied",
        { mcp_servers: [{ ...authServer, status: "running" }] },
        "ready",
      ],
    ])("Should map %s to %s [UT-031]", (_case, overrides, expected) => {
      expect(marketplaceSetupState({ entry, item: item(overrides), inventoryPending: false })).toBe(
        expected
      );
    });

    // UT-033 [US-007.AC-2, US-007.EC-1]: disabled reads Turn on; unknown inventory is never Ready.
    it("Should map disabled to Turn on and a pending inventory to unknown [UT-033]", () => {
      expect(
        marketplaceSetupState({ entry, item: item({ enabled: false }), inventoryPending: false })
      ).toBe("turn-on");
      expect(marketplaceSetupState({ entry, item: null, inventoryPending: true })).toBe("unknown");
      expect(
        marketplaceSetupState({
          entry,
          item: item({ enabled: false }),
          inventoryPending: false,
          enableFailed: true,
        })
      ).toBe("enable-failed");
      expect(
        marketplaceSetupState({ entry: { installed: false }, item: null, inventoryPending: true })
      ).toBe("add");
      expect(
        marketplaceSetupState({ entry, item: item({}), inventoryPending: false, pending: true })
      ).toBe("adding");
    });
  });

  // UT-034 [US-013.AC-2]: preset sources read by name; custom sources keep theirs.
  it("Should name preset sources plainly and keep custom source names", () => {
    expect(marketplaceSourceLabel("claude-plugins-official")).toBe("Claude plugins");
    expect(marketplaceSourceLabel("team-plugins")).toBe("team-plugins");
  });
});
