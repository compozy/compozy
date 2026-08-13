import { describe, expect, it } from "vitest";

import { isCommitScopeEmpty, toWorktreeExitLadder } from "../worktree-exit-ladder";
import type { WorktreeExitPlanPayload } from "../../types";

/**
 * UT-130 — the exit ladder is the daemon's, rendered verbatim.
 *
 * Invariant: the web performs no selection, ordering, filtering or label
 * derivation. Owning layer: this projection, which every exit surface reads.
 */
function plan(overrides: Partial<WorktreeExitPlanPayload> = {}): WorktreeExitPlanPayload {
  return {
    worktree_id: "wt_1",
    actions: [],
    commit_scope: {
      changed_files: 0,
      insertions: 0,
      deletions: 0,
      untracked_files: [],
      untracked_total: 0,
    },
    cleanup: { safe: false },
    ...overrides,
  } as WorktreeExitPlanPayload;
}

function action(
  overrides: Partial<WorktreeExitPlanPayload["actions"][number]>
): WorktreeExitPlanPayload["actions"][number] {
  return { action: "commit", label: "Commit", enabled: true, ...overrides } as never;
}

describe("toWorktreeExitLadder", () => {
  it("Should take the primary from the payload at every ladder position", () => {
    const positions = ["commit", "commit_push", "push", "open_pr", "view_pr"] as const;
    for (const primary of positions) {
      const ladder = toWorktreeExitLadder(
        plan({
          primary,
          actions: positions.map(entry =>
            action({ action: entry, label: `label:${entry}`, enabled: entry === primary })
          ),
        })
      );
      expect(ladder.primary?.action).toBe(primary);
      expect(ladder.primary?.label).toBe(`label:${primary}`);
      expect(ladder.menuRows.map(row => row.action)).not.toContain(primary);
    }
  });

  it("Should keep the payload's row order rather than sorting or filtering", () => {
    const ladder = toWorktreeExitLadder(
      plan({
        primary: "push",
        actions: [
          action({ action: "view_pr", label: "View", enabled: false }),
          action({ action: "push", label: "Push" }),
          action({ action: "commit", label: "Commit", enabled: false }),
        ],
      })
    );
    expect(ladder.rows.map(row => row.action)).toEqual(["view_pr", "push", "commit"]);
  });

  it("Should render each blocked reason verbatim", () => {
    const reason = "Branch has diverged from upstream. Rebase/merge first.";
    const ladder = toWorktreeExitLadder(
      plan({
        primary: "commit",
        actions: [
          action({ action: "commit", label: "Commit" }),
          action({ action: "push", label: "Push", enabled: false, blocked_reason: reason }),
        ],
      })
    );
    expect(ladder.menuRows[0]?.blockedReason).toBe(reason);
  });

  it("Should block the whole control on a global pause and name its cause", () => {
    for (const cause of [
      "Git actions are paused while a bound session is running.",
      "Git status could not be read for this worktree.",
    ]) {
      const ladder = toWorktreeExitLadder(
        plan({
          primary: "commit",
          global_pause_cause: cause,
          actions: [
            action({ action: "commit", label: "Commit", enabled: false, blocked_reason: cause }),
          ],
        })
      );
      expect(ladder.blocked).toBe(true);
      expect(ladder.blockedReason).toBe(cause);
    }
  });

  it("Should report no pause when the plan names none", () => {
    const ladder = toWorktreeExitLadder(
      plan({ primary: "commit", actions: [action({ action: "commit", label: "Commit" })] })
    );
    expect(ladder.blocked).toBe(false);
    expect(ladder.blockedReason).toBeUndefined();
  });

  it("Should treat an absent forge as absent affordances rather than disabled ones", () => {
    const ladder = toWorktreeExitLadder(
      plan({ primary: "commit", actions: [action({ action: "commit", label: "Commit" })] })
    );
    expect(ladder.forge).toBeUndefined();
    expect(ladder.hasForgeStatus).toBe(false);
    expect(ladder.rows.some(row => row.action === "open_pr")).toBe(false);
  });

  it("Should route only the actions that need input through a dialog", () => {
    const ladder = toWorktreeExitLadder(
      plan({
        primary: "commit",
        actions: [
          action({ action: "commit", label: "Commit" }),
          action({ action: "commit_push", label: "Commit & push" }),
          action({ action: "push", label: "Push" }),
          action({ action: "open_pr", label: "Open PR" }),
          action({ action: "view_pr", label: "View PR", url: "https://example.test/pr/1" }),
        ],
      })
    );
    const byAction = Object.fromEntries(ladder.rows.map(row => [row.action, row]));
    expect(byAction.commit.requiresInput).toBe(true);
    expect(byAction.commit_push.requiresInput).toBe(true);
    expect(byAction.open_pr.requiresInput).toBe(true);
    expect(byAction.push.requiresInput).toBe(false);
    expect(byAction.view_pr.opensURL).toBe(true);
  });
});

describe("isCommitScopeEmpty", () => {
  it("Should count untracked additions as scope", () => {
    expect(
      isCommitScopeEmpty({
        changed_files: 0,
        insertions: 0,
        deletions: 0,
        untracked_files: ["a.ts"],
        untracked_total: 1,
      })
    ).toBe(false);
  });

  it("Should be empty only when nothing at all would stage", () => {
    expect(
      isCommitScopeEmpty({
        changed_files: 0,
        insertions: 0,
        deletions: 0,
        untracked_files: [],
        untracked_total: 0,
      })
    ).toBe(true);
  });
});
