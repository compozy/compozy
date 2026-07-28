import { describe, expect, it } from "vitest";

import {
  memoriesListOptions,
  memoryDecisionsOptions,
  memoryDetailOptions,
  memorySearchOptions,
} from "@/systems/knowledge/lib/query-options";

describe("memoriesListOptions", () => {
  it("Should expose infinite cursor pages without automatic catalog polling", () => {
    const options = memoriesListOptions({ scope: "global" });
    expect(options.staleTime).toBe(30_000);
    expect(options.refetchInterval).toBeUndefined();
    expect(options.initialPageParam).toBeUndefined();
    const page = {
      memories: [],
      page: { has_more: true, limit: 1, next_cursor: "memory-next", total: 2 },
    };
    expect(options.getNextPageParam?.(page, [page], undefined, [undefined])).toBe("memory-next");
  });

  it("Should include the full selector tuple in the query key", () => {
    const options = memoriesListOptions({
      scope: "agent",
      agentName: "cto",
      agentTier: "workspace",
      workspaceId: "ws_launch",
      type: "reference",
      sort: "name",
      includeSystem: false,
      limit: 25,
    });
    expect(options.queryKey).toEqual([
      "knowledge",
      "list",
      "agent",
      "ws_launch",
      "cto",
      "workspace",
      "reference",
      "name",
      false,
      25,
    ]);
  });

  it("Should pad missing selectors with empty strings", () => {
    const options = memoriesListOptions();
    expect(options.queryKey).toEqual([
      "knowledge",
      "list",
      "global",
      "",
      "",
      "",
      "",
      "",
      null,
      null,
    ]);
  });
});

describe("memoryDetailOptions", () => {
  it("Should include staleTime defaults", () => {
    const options = memoryDetailOptions({ scope: "global" }, "test.md");
    expect(options.staleTime).toBe(30_000);
  });

  it("Should include scope, filename, and selector tuple in the query key", () => {
    const options = memoryDetailOptions({ scope: "global", workspaceId: "ws" }, "user_role.md");
    expect(options.queryKey).toEqual([
      "knowledge",
      "detail",
      "user_role.md",
      "global",
      "ws",
      "",
      "",
    ]);
  });

  it("Should be disabled when selector is missing", () => {
    const options = memoryDetailOptions(undefined, "test.md");
    expect(options.enabled).toBe(false);
  });

  it("Should be disabled when filename is empty", () => {
    const options = memoryDetailOptions({ scope: "global" }, "");
    expect(options.enabled).toBe(false);
  });

  it("Should be enabled when both selector and filename are provided", () => {
    const options = memoryDetailOptions({ scope: "global" }, "test.md");
    expect(options.enabled).toBe(true);
  });
});

describe("memorySearchOptions", () => {
  it("Should be disabled when query text is empty", () => {
    const options = memorySearchOptions({ scope: "global" }, "");
    expect(options.enabled).toBe(false);
  });

  it("Should be enabled when selector and query text are present", () => {
    const options = memorySearchOptions({ scope: "global" }, "launch");
    expect(options.enabled).toBe(true);
    expect(options.queryKey).toEqual([
      "knowledge",
      "search",
      "launch",
      "global",
      "",
      "",
      "",
      null,
      null,
      null,
    ]);
  });

  it("Should propagate workspace selector to the query key", () => {
    const options = memorySearchOptions(
      { scope: "workspace", workspaceId: "ws_launch" },
      "rollout"
    );
    expect(options.queryKey).toEqual([
      "knowledge",
      "search",
      "rollout",
      "workspace",
      "ws_launch",
      "",
      "",
      null,
      null,
      null,
    ]);
  });

  it("Should include response-shaping search options in the query key", () => {
    const options = memorySearchOptions({ scope: "global" }, "rollout", {
      topK: 7,
      includeSystem: true,
      explain: true,
    });

    expect(options.queryKey.slice(-3)).toEqual([7, true, true]);
  });
});

describe("memoryDecisionsOptions", () => {
  it("Should be disabled when no params are provided", () => {
    expect(memoryDecisionsOptions(undefined).enabled).toBe(false);
  });

  it("Should be enabled when scope is set and include selector + filename in key", () => {
    const options = memoryDecisionsOptions({
      scope: "agent",
      agentName: "cto",
      agentTier: "workspace",
      workspaceId: "ws_launch",
      filename: "launch.md",
      op: "update",
      since: "2026-04-25T21:00:00Z",
      limit: 10,
    });
    expect(options.enabled).toBe(true);
    expect(options.queryKey).toEqual([
      "knowledge",
      "decisions",
      "agent",
      "ws_launch",
      "cto",
      "workspace",
      "launch.md",
      "update",
      "2026-04-25T21:00:00Z",
      10,
    ]);
  });
});
