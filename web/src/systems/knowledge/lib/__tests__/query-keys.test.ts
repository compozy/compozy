import { describe, expect, it } from "vitest";

import { knowledgeKeys } from "../query-keys";

describe("knowledgeKeys", () => {
  it("Should expose hierarchical keys for list, detail, search and decisions", () => {
    expect(knowledgeKeys.all).toEqual(["knowledge"]);
    expect(knowledgeKeys.lists()).toEqual(["knowledge", "list"]);
    expect(knowledgeKeys.list({ scope: "global" })).toEqual([
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
    expect(
      knowledgeKeys.list({
        scope: "agent",
        workspaceId: "ws_launch",
        agentName: "cto",
        agentTier: "workspace",
        type: "reference",
        sort: "recent",
        includeSystem: true,
        cursor: "ignored-cursor",
        limit: 25,
      })
    ).toEqual([
      "knowledge",
      "list",
      "agent",
      "ws_launch",
      "cto",
      "workspace",
      "reference",
      "recent",
      true,
      25,
    ]);
    expect(knowledgeKeys.list({ scope: "global", cursor: "first" })).toEqual(
      knowledgeKeys.list({ scope: "global", cursor: "second" })
    );
    expect(knowledgeKeys.list({ scope: "global", sort: "recent" })).not.toEqual(
      knowledgeKeys.list({ scope: "global", sort: "name" })
    );

    expect(knowledgeKeys.details()).toEqual(["knowledge", "detail"]);
    expect(knowledgeKeys.detail("user_role.md", { scope: "global" })).toEqual([
      "knowledge",
      "detail",
      "user_role.md",
      "global",
      "",
      "",
      "",
    ]);

    expect(knowledgeKeys.searches()).toEqual(["knowledge", "search"]);
    expect(
      knowledgeKeys.search("launch", {
        scope: "workspace",
        workspaceId: "ws_launch",
      })
    ).toEqual([
      "knowledge",
      "search",
      "launch",
      "workspace",
      "ws_launch",
      "",
      "",
      null,
      null,
      null,
    ]);

    expect(knowledgeKeys.decisions()).toEqual(["knowledge", "decisions"]);
    expect(
      knowledgeKeys.decisionsFor({
        scope: "global",
        filename: "global/user.md",
        op: "update",
        since: "2026-04-25T21:00:00Z",
        limit: 10,
      })
    ).toEqual([
      "knowledge",
      "decisions",
      "global",
      "",
      "",
      "",
      "global/user.md",
      "update",
      "2026-04-25T21:00:00Z",
      10,
    ]);
  });

  it("Should pad missing selector segments with empty strings", () => {
    expect(knowledgeKeys.list()).toEqual([
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
    expect(knowledgeKeys.detail("test.md")).toEqual([
      "knowledge",
      "detail",
      "test.md",
      "",
      "",
      "",
      "",
    ]);
  });

  it("Should keep list and detail keys rooted at the all key", () => {
    expect(knowledgeKeys.list({ scope: "global" })[0]).toBe(knowledgeKeys.all[0]);
    expect(knowledgeKeys.detail("test.md", { scope: "global" })[0]).toBe(knowledgeKeys.all[0]);
    expect(knowledgeKeys.search("x", { scope: "global" })[0]).toBe(knowledgeKeys.all[0]);
    expect(knowledgeKeys.decisionsFor({ scope: "global", filename: "test.md" })[0]).toBe(
      knowledgeKeys.all[0]
    );
  });

  it("Should isolate search and decision variants that produce different server responses", () => {
    expect(
      knowledgeKeys.search("launch", { scope: "global" }, { topK: 3, includeSystem: false })
    ).not.toEqual(
      knowledgeKeys.search("launch", { scope: "global" }, { topK: 8, includeSystem: true })
    );
    expect(
      knowledgeKeys.decisionsFor({ scope: "global", filename: "one.md", limit: 10 })
    ).not.toEqual(knowledgeKeys.decisionsFor({ scope: "global", filename: "two.md", limit: 10 }));
  });
});
