import { describe, expect, it } from "vitest";

import { knowledgeKeys } from "../query-keys";

describe("knowledgeKeys", () => {
  it("Should expose hierarchical keys for list, detail, search and decisions", () => {
    expect(knowledgeKeys.all).toEqual(["knowledge"]);
    expect(knowledgeKeys.lists()).toEqual(["knowledge", "list"]);
    expect(knowledgeKeys.list({ scope: "profile" })).toEqual([
      "knowledge",
      "list",
      "",
      "profile",
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
        profile: "marketing",
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
      "marketing",
      "agent",
      "ws_launch",
      "cto",
      "workspace",
      "reference",
      "recent",
      true,
      25,
    ]);
    expect(knowledgeKeys.list({ scope: "profile", cursor: "first" })).toEqual(
      knowledgeKeys.list({ scope: "profile", cursor: "second" })
    );
    expect(knowledgeKeys.list({ scope: "profile", sort: "recent" })).not.toEqual(
      knowledgeKeys.list({ scope: "profile", sort: "name" })
    );

    expect(knowledgeKeys.details()).toEqual(["knowledge", "detail"]);
    expect(knowledgeKeys.detail("user_role.md", { scope: "profile" })).toEqual([
      "knowledge",
      "detail",
      "user_role.md",
      "",
      "profile",
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
      "",
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
        scope: "profile",
        filename: "global/user.md",
        op: "update",
        since: "2026-04-25T21:00:00Z",
        limit: 10,
      })
    ).toEqual([
      "knowledge",
      "decisions",
      "",
      "profile",
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
      "",
      "profile",
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
      "",
    ]);
  });

  it("Should keep list and detail keys rooted at the all key", () => {
    expect(knowledgeKeys.list({ scope: "profile" })[0]).toBe(knowledgeKeys.all[0]);
    expect(knowledgeKeys.detail("test.md", { scope: "profile" })[0]).toBe(knowledgeKeys.all[0]);
    expect(knowledgeKeys.search("x", { scope: "profile" })[0]).toBe(knowledgeKeys.all[0]);
    expect(knowledgeKeys.decisionsFor({ scope: "profile", filename: "test.md" })[0]).toBe(
      knowledgeKeys.all[0]
    );
  });

  it("Should isolate search and decision variants that produce different server responses", () => {
    expect(
      knowledgeKeys.search("launch", { scope: "profile" }, { topK: 3, includeSystem: false })
    ).not.toEqual(
      knowledgeKeys.search("launch", { scope: "profile" }, { topK: 8, includeSystem: true })
    );
    expect(
      knowledgeKeys.decisionsFor({ scope: "profile", filename: "one.md", limit: 10 })
    ).not.toEqual(knowledgeKeys.decisionsFor({ scope: "profile", filename: "two.md", limit: 10 }));
  });

  it("Should isolate every profile-bound cache family by named profile", () => {
    const marketing = { scope: "profile", profile: "marketing" } as const;
    const engineering = { scope: "profile", profile: "engineering" } as const;

    expect(knowledgeKeys.list(marketing)).not.toEqual(knowledgeKeys.list(engineering));
    expect(knowledgeKeys.detail("operator.md", marketing)).not.toEqual(
      knowledgeKeys.detail("operator.md", engineering)
    );
    expect(knowledgeKeys.search("launch", marketing)).not.toEqual(
      knowledgeKeys.search("launch", engineering)
    );
    expect(knowledgeKeys.decisionsFor(marketing)).not.toEqual(
      knowledgeKeys.decisionsFor(engineering)
    );
  });
});
