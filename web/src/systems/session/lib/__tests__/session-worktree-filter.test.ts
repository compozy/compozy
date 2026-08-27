// Suite: session list worktree scoping
// Invariant: a worktree scope is a server-side filter that enters the query key, so two window
// scopes produce two distinct session queries and a fallback removes the filter entirely rather
// than sending an empty one.
// Owning layer: session query layer. Canonical suite: this lib test.
import { describe, expect, it } from "vitest";

import { sessionKeys } from "../query-keys";
import { normalizeSessionListFilters, sessionListRequest } from "../session-list-query";

describe("session worktree scoping", () => {
  it("Should normalize a governed root and keep it in requests and query keys", () => {
    const filters = normalizeSessionListFilters({ workspace_id: "ws_alpha", root: " ses_root " });

    expect(filters.root).toBe("ses_root");
    expect(sessionListRequest(filters, "cursor-2")).toMatchObject({
      root: "ses_root",
      cursor: "cursor-2",
    });
    expect(sessionKeys.list(filters)).not.toEqual(sessionKeys.list({ workspace_id: "ws_alpha" }));
  });

  it("Should keep an explicit empty workspace pin instead of widening to every workspace", () => {
    expect(normalizeSessionListFilters({ workspace_id: "" })).toEqual({ workspace_id: "" });
    expect(normalizeSessionListFilters({ all_workspaces: true })).toMatchObject({
      all_workspaces: true,
    });
  });

  it("Should drop a blank governed root instead of widening with an empty value", () => {
    expect(
      normalizeSessionListFilters({ workspace_id: "ws_alpha", root: "   " })
    ).not.toHaveProperty("root");
  });

  it("Should carry a worktree filter through normalization", () => {
    const filters = normalizeSessionListFilters({
      workspace_id: "ws_alpha",
      worktree: "wt_payments",
    });

    expect(filters.worktree).toBe("wt_payments");
  });

  it("Should drop an absent or blank worktree instead of sending an empty filter", () => {
    expect(normalizeSessionListFilters({ workspace_id: "ws_alpha" })).not.toHaveProperty(
      "worktree"
    );
    expect(
      normalizeSessionListFilters({ workspace_id: "ws_alpha", worktree: "   " })
    ).not.toHaveProperty("worktree");
  });

  it("Should give two window scopes two distinct query keys", () => {
    const windowOne = sessionKeys.list({ workspace_id: "ws_alpha", worktree: "wt_payments" });
    const windowTwo = sessionKeys.list({ workspace_id: "ws_alpha", worktree: "wt_auth" });

    // Distinct keys are what stop one window's list from serving another's.
    expect(windowOne).not.toEqual(windowTwo);
  });

  it("Should give a fallback scope the same key as the unscoped workspace", () => {
    const fallback = sessionKeys.list({ workspace_id: "ws_alpha", worktree: undefined });
    const unscoped = sessionKeys.list({ workspace_id: "ws_alpha" });

    // A selection that stopped resolving reads exactly like the parent, which
    // is what the menubar notice promises.
    expect(fallback).toEqual(unscoped);
  });

  it("Should send the worktree with every paged request", () => {
    const filters = normalizeSessionListFilters({
      workspace_id: "ws_alpha",
      worktree: "wt_payments",
    });

    const page = sessionListRequest(filters, "cursor-2");

    // The filter rides the cursor, so pagination cannot straddle a scope change.
    expect(page.worktree).toBe("wt_payments");
    expect(page.cursor).toBe("cursor-2");
  });

  it("Should keep named and aggregate profile catalogs in distinct keys and requests", () => {
    const named = normalizeSessionListFilters({ all_workspaces: true, profile: " marketing " });
    const aggregate = normalizeSessionListFilters({ all_workspaces: true, all_profiles: true });

    expect(sessionKeys.list(named)).not.toEqual(sessionKeys.list(aggregate));
    expect(sessionListRequest(named)).toMatchObject({ profile: "marketing" });
    expect(sessionListRequest(aggregate)).toMatchObject({ all_profiles: true });
  });
});
