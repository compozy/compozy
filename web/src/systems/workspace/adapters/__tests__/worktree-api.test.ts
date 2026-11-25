// Invariant: createWorktree serializes the optional profile selector without changing its body.
// Owning layer: workspace worktree HTTP adapter. No existing suite owns this adapter.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { expectFetchRequest, mockJsonResponse } from "@/test/fetch-test-utils";
import { buildWorktreeFixture } from "../../mocks/worktree-fixtures";

import {
  cancelWorktreeCreate,
  createWorktree,
  dismissWorktree,
  fetchWorktrees,
  removeWorktree,
} from "../worktree-api";

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn());
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("createWorktree", () => {
  it.each([
    {
      label: "selected profile",
      profile: "marketing",
      path: "/api/workspaces/ws_alpha/worktrees?profile=marketing",
    },
    {
      label: "omitted profile",
      profile: undefined,
      path: "/api/workspaces/ws_alpha/worktrees",
    },
  ])("Should serialize the $label", async ({ profile, path }) => {
    const worktree = buildWorktreeFixture();
    mockJsonResponse({ worktree }, { status: 202 });

    await expect(createWorktree("ws_alpha", { name: "docs-refresh" }, profile)).resolves.toEqual(
      worktree
    );
    await expectFetchRequest({
      body: { name: "docs-refresh" },
      method: "POST",
      path,
    });
  });
});

// The shared list must retain tombstones used by session binding resolution;
// visible catalog rows remain filtered by worktree-display.
describe("fetchWorktrees", () => {
  it("Should retain removed bindings while refreshing", async () => {
    const removed = buildWorktreeFixture({ state: "removed" });
    mockJsonResponse({ worktrees: [removed], discovered: [] });
    await expect(fetchWorktrees("ws_alpha", { refresh: true })).resolves.toMatchObject({
      worktrees: [removed],
    });
    await expectFetchRequest({
      method: "GET",
      path: "/api/workspaces/ws_alpha/worktrees?refresh=true&include_removed=true",
    });
  });

  // Regression test for AST-008: worktree mutations targeting a remote computer must route
  // through `/c/{computerId}` rather than the local controller.
  it("AST-008: Should route worktree create, cancel, dismiss, and remove through /c/{computerId}", async () => {
    const worktree = buildWorktreeFixture({ id: "wt_remote_1" });
    mockJsonResponse({ worktree }, { status: 202 });
    await expect(
      createWorktree("ws_alpha", { name: "remote-wt" }, undefined, undefined, "cmp_remote")
    ).resolves.toEqual(worktree);
    await expectFetchRequest({
      body: { name: "remote-wt" },
      method: "POST",
      path: "/c/cmp_remote/api/workspaces/ws_alpha/worktrees",
    });

    mockJsonResponse({}, { status: 204 });
    await cancelWorktreeCreate("ws_alpha", "wt_remote_1", undefined, "cmp_remote");
    await expectFetchRequest({
      method: "POST",
      path: "/c/cmp_remote/api/workspaces/ws_alpha/worktrees/wt_remote_1/cancel",
    });

    mockJsonResponse({ branch: null }, { status: 200 });
    await dismissWorktree("ws_alpha", "wt_remote_1", { computerId: "cmp_remote" });
    await expectFetchRequest({
      method: "POST",
      path: "/c/cmp_remote/api/workspaces/ws_alpha/worktrees/wt_remote_1/dismiss",
    });

    mockJsonResponse({ branch: null }, { status: 200 });
    await removeWorktree("ws_alpha", "wt_remote_1", { computerId: "cmp_remote" });
    await expectFetchRequest({
      method: "DELETE",
      path: "/c/cmp_remote/api/workspaces/ws_alpha/worktrees/wt_remote_1",
    });
  });
});
