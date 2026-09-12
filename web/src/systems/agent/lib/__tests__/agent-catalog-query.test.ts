import { describe, expect, it } from "vitest";

import { agentCatalogRequest, normalizeAgentCatalogFilter } from "../agent-catalog-query";
import { agentKeys } from "../query-keys";

describe("agent-catalog-query", () => {
  it.each([
    { profile: undefined, expected: "default" },
    { profile: "  ", expected: "default" },
    { profile: " open-design ", expected: "open-design" },
  ])(
    "Should normalize profile '$profile' and filter text identically for requests and cache keys",
    ({ profile, expected }) => {
      expect(normalizeAgentCatalogFilter({ profile, q: " ", category: "  ", name: " " })).toEqual({
        profile: expected,
        name: undefined,
        q: undefined,
        category: undefined,
        status: undefined,
        limit: undefined,
      });
      const filters = {
        profile,
        name: "fraud-ops-agent",
        q: " ",
        category: "Ops",
        status: "active" as const,
      };
      const normalized = {
        profile: expected,
        name: "fraud-ops-agent",
        q: undefined,
        category: "Ops",
        status: "active",
        limit: undefined,
      };
      expect(agentCatalogRequest(" ws_alpha ", filters)).toEqual({
        workspace: "ws_alpha",
        ...normalized,
      });
      expect(agentKeys.catalog(" ws_alpha ", filters)).toEqual([
        "agents",
        "catalog",
        "ws_alpha",
        normalized,
      ]);
    }
  );
});
