import { describe, expect, it } from "vitest";

import {
  environmentCreateFields,
  environmentTargetLabel,
  isEnvironmentTargetMissing,
  selectableWorktrees,
  WORKSPACE_ROOT_LABEL,
} from "../session-environment-target";
import type { WorktreePayload } from "@/systems/workspace";

/**
 * UT-132 — the session-create environment choice.
 *
 * Invariant: only a ready worktree is an offer, a selection that stops being
 * ready blocks rather than silently falling back, and "new" never reaches the
 * create request as an inline field. Owning layer: this view model, shared by
 * the field, the chip and the fork dialog.
 */
function worktree(overrides: Partial<WorktreePayload> = {}): WorktreePayload {
  return {
    id: "wt_ready",
    workspace_id: "ws_1",
    name: "payments-retry",
    branch: "pedro/payments-retry",
    path: "/tmp/payments-retry",
    state: "ready",
    origin: "manual",
    setup_state: "ok",
    created_branch: true,
    dirty: null,
    ahead: null,
    behind: null,
    agent_activity: "idle",
    created_at: "2026-08-12T00:00:00Z",
    updated_at: "2026-08-12T00:00:00Z",
    ...overrides,
  } as WorktreePayload;
}

describe("selectableWorktrees", () => {
  it("Should offer ready worktrees only", () => {
    const rows = [
      worktree(),
      worktree({ id: "wt_pending", state: "pending" }),
      worktree({ id: "wt_missing", state: "missing" }),
      worktree({ id: "wt_failed", state: "failed" }),
    ];
    expect(selectableWorktrees(rows).map(entry => entry.id)).toEqual(["wt_ready"]);
  });
});

describe("environmentTargetLabel", () => {
  it("Should name the workspace root with the locked vocabulary", () => {
    expect(environmentTargetLabel({ kind: "root" }, [])).toBe(WORKSPACE_ROOT_LABEL);
  });

  it("Should use the worktree's record label, never its branch", () => {
    expect(environmentTargetLabel({ kind: "worktree", worktreeId: "wt_ready" }, [worktree()])).toBe(
      "payments-retry"
    );
  });

  it("Should resolve to nothing when the selected worktree is gone", () => {
    expect(
      environmentTargetLabel({ kind: "worktree", worktreeId: "wt_ready" }, [])
    ).toBeUndefined();
  });
});

describe("isEnvironmentTargetMissing", () => {
  it("Should flag a selection whose record disappeared", () => {
    expect(isEnvironmentTargetMissing({ kind: "worktree", worktreeId: "wt_ready" }, [])).toBe(true);
  });

  it("Should flag a selection that stopped being ready", () => {
    expect(
      isEnvironmentTargetMissing({ kind: "worktree", worktreeId: "wt_ready" }, [
        worktree({ state: "missing" }),
      ])
    ).toBe(true);
  });

  it("Should not flag the workspace root", () => {
    expect(isEnvironmentTargetMissing({ kind: "root" }, [])).toBe(false);
  });
});

describe("environmentCreateFields", () => {
  it("Should send no worktree for the workspace root", () => {
    expect(environmentCreateFields({ kind: "root" })).toEqual({});
  });

  it("Should bind an existing worktree by id", () => {
    expect(environmentCreateFields({ kind: "worktree", worktreeId: "wt_ready" })).toEqual({
      worktree: "wt_ready",
    });
  });

  it("Should refuse to send a new-worktree intent before one exists", () => {
    expect(environmentCreateFields({ kind: "new", name: "docs-refresh" })).toEqual({});
  });

  it("Should bind the materialized worktree once it is ready", () => {
    expect(environmentCreateFields({ kind: "new", name: "docs-refresh" }, "wt_new")).toEqual({
      worktree: "wt_new",
    });
  });
});
