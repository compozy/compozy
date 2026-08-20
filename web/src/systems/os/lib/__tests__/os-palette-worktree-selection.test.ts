// Suite: palette worktree selection
// Invariant: a global worktree row switches the active workspace before the
// per-window worktree scope is written; a workspace-scoped row only scopes.
// Owning layer: unit — the selection helper.
import { beforeEach, describe, expect, it, vi } from "vitest";

import { applyPaletteWorktreeSelection } from "../os-palette-worktree-selection";

const mocks = vi.hoisted(() => ({
  setActiveWorkspaceId: vi.fn(),
  selectWorktreeForScope: vi.fn(),
}));

vi.mock("@/systems/workspace", () => ({
  setActiveWorkspaceId: mocks.setActiveWorkspaceId,
  selectWorktreeForScope: mocks.selectWorktreeForScope,
}));

beforeEach(() => {
  mocks.setActiveWorkspaceId.mockReset();
  mocks.selectWorktreeForScope.mockReset();
});

describe("applyPaletteWorktreeSelection", () => {
  it("Should switch workspace before scoping a global worktree row", () => {
    applyPaletteWorktreeSelection({
      scope: "global",
      worktreeScopeId: "window:1",
      activeWorkspaceId: null,
      entry: { workspaceId: "ws-b", worktree: { id: "wt-9" } },
    });
    expect(mocks.setActiveWorkspaceId).toHaveBeenCalledExactlyOnceWith("ws-b");
    expect(mocks.selectWorktreeForScope).toHaveBeenCalledExactlyOnceWith(
      "window:1",
      "ws-b",
      "wt-9"
    );
  });

  it("Should keep the current workspace when the globe is already off", () => {
    applyPaletteWorktreeSelection({
      scope: "workspace",
      worktreeScopeId: "shell",
      activeWorkspaceId: "ws-a",
      entry: { worktree: { id: "wt-1" } },
    });
    expect(mocks.setActiveWorkspaceId).not.toHaveBeenCalled();
    expect(mocks.selectWorktreeForScope).toHaveBeenCalledExactlyOnceWith("shell", "ws-a", "wt-1");
  });
});
