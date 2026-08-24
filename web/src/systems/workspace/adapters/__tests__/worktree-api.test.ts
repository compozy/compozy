// Invariant: createWorktree serializes the optional profile selector without changing its body.
// Owning layer: workspace worktree HTTP adapter. No existing suite owns this adapter.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { expectFetchRequest, mockJsonResponse } from "@/test/fetch-test-utils";
import { buildWorktreeFixture } from "../../mocks/worktree-fixtures";

import { createWorktree } from "../worktree-api";

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
