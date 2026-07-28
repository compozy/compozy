import { describe, expect, it } from "vitest";

import { agentCatalogRequest, normalizeAgentCatalogFilter } from "../agent-catalog-query";

describe("agent-catalog-query", () => {
  it("Should normalize blank filter text the same way for requests and cache keys", () => {
    expect(normalizeAgentCatalogFilter({ q: " ", category: "  ", name: " " })).toEqual({
      name: undefined,
      q: undefined,
      category: undefined,
      status: undefined,
      limit: undefined,
    });
    expect(
      agentCatalogRequest(" ws_alpha ", {
        name: "fraud-ops-agent",
        q: " ",
        category: "Ops",
        status: "active",
      })
    ).toEqual({
      workspace: "ws_alpha",
      name: "fraud-ops-agent",
      q: undefined,
      category: "Ops",
      status: "active",
      limit: undefined,
    });
  });
});
