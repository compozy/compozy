import { describe, expect, it } from "vitest";

import {
  agentCatalogRequest,
  flattenAgentAggregatePages,
  keepCatalogAcrossSearch,
  normalizeAgentCatalogFilter,
} from "../agent-catalog-query";
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

  // T-33 (web): the aggregate's identity is (workspace_id, name), so a page boundary
  // between same-named agents of two projects never collapses them.
  it("Should flatten aggregate pages by project and name", () => {
    const item = (name: string, workspaceId?: string) => ({
      agent: { name, workspace_id: workspaceId } as never,
    });
    const page = (agents: ReturnType<typeof item>[]) => ({
      agents,
      facets: { active: 0, categories: [], idle: 0, total: 3 },
      page: { has_more: false, limit: 1, total: 3 },
      sessions_available: true,
    });
    const flattened = flattenAgentAggregatePages({
      pages: [
        page([item("reviewer")]),
        page([item("reviewer", "ws_a")]),
        page([item("reviewer", "ws_a")]),
      ],
      pageParams: [undefined, "c1", "c2"],
    } as never);
    expect(flattened.map(entry => entry.agent.workspace_id ?? "")).toEqual(["", "ws_a"]);
  });

  // A search edit keeps the previous rows on screen; a change of scope or population never does.
  it("Should keep the previous catalog rows across a search edit only", () => {
    const rows = ["rows"];
    const keyOf = (workspace: string, filters: Parameters<typeof agentKeys.catalog>[1] = {}) =>
      agentKeys.catalog(workspace, filters);

    expect(
      keepCatalogAcrossSearch(rows, keyOf("ws_a", { q: "rev" }), keyOf("ws_a", { q: "revi" }))
    ).toBe(rows);
    expect(keepCatalogAcrossSearch(rows, keyOf("ws_a"), keyOf("ws_a", { q: "rev" }))).toBe(rows);
    expect(keepCatalogAcrossSearch(rows, undefined, keyOf("ws_a"))).toBeUndefined();
    // Another workspace, profile, category or status is another population: nothing stale shows.
    expect(keepCatalogAcrossSearch(rows, keyOf("ws_a"), keyOf("ws_b"))).toBeUndefined();
    expect(
      keepCatalogAcrossSearch(rows, keyOf("ws_a"), keyOf("ws_a", { profile: "growth" }))
    ).toBeUndefined();
    expect(
      keepCatalogAcrossSearch(rows, keyOf("ws_a"), keyOf("ws_a", { category: "Ops" }))
    ).toBeUndefined();
    expect(
      keepCatalogAcrossSearch(rows, keyOf("ws_a"), keyOf("ws_a", { status: "active" }))
    ).toBeUndefined();
    // The All projects aggregate follows the same rule, per Project-filter selection.
    expect(
      keepCatalogAcrossSearch(
        rows,
        agentKeys.aggregate(["ws_a"], { q: "rev" }),
        agentKeys.aggregate(["ws_a"], { q: "revi" })
      )
    ).toBe(rows);
    expect(
      keepCatalogAcrossSearch(
        rows,
        agentKeys.aggregate(["ws_a"], { q: "rev" }),
        agentKeys.aggregate(["ws_a", "ws_b"], { q: "rev" })
      )
    ).toBeUndefined();
  });
});
