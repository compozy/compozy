import { describe, expect, it } from "vitest";

import { toWorktreePrDialogModel } from "../worktree-pr-rows";
import { toWorktreeExitLadder } from "../worktree-exit-ladder";
import type { WorktreeExitPlanPayload } from "../../types";

/**
 * Invariant: the PR step offers only what the plan authorises, and its inputs
 * carry only daemon-supplied text. Owning layer: this projection, which the PR
 * dialog renders without adding rows or content of its own.
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

const FORGE = {
  provider: "github",
  request_noun: "PR",
  open_action_label: "Open PR",
  view_action_label: "View PR",
  supports_draft: true,
} as WorktreeExitPlanPayload["forge"];

describe("toWorktreePrDialogModel", () => {
  it("Should offer draft as a peer of create when the provider supports it", () => {
    const model = toWorktreePrDialogModel(
      toWorktreeExitLadder(
        plan({
          forge: FORGE,
          actions: [{ action: "open_pr", label: "Open PR", enabled: true }],
        })
      )
    );
    expect(model.mode).toBe("create");
    expect(model.rows.map(row => row.key)).toEqual(["draft", "create"]);
    expect(model.rows[0]?.label).toBe("Open PR (draft)");
    expect(model.rows.find(row => row.key === "create")?.primary).toBe(true);
  });

  it("Should drop the draft row when the provider does not support drafts", () => {
    const model = toWorktreePrDialogModel(
      toWorktreeExitLadder(
        plan({
          forge: { ...FORGE, supports_draft: false } as WorktreeExitPlanPayload["forge"],
          actions: [{ action: "open_pr", label: "Open PR", enabled: true }],
        })
      )
    );
    expect(model.rows.map(row => row.key)).toEqual(["create"]);
  });

  it("Should collapse to a single view row when a request is already open", () => {
    const model = toWorktreePrDialogModel(
      toWorktreeExitLadder(
        plan({
          forge: FORGE,
          actions: [
            {
              action: "view_pr",
              label: "View PR",
              enabled: true,
              pr_number: 412,
              url: "https://example.test/pr/412",
            },
          ],
        })
      )
    );
    expect(model.mode).toBe("existing");
    expect(model.rows).toHaveLength(1);
    expect(model.rows[0]?.key).toBe("view");
  });

  it("Should leave only the browser row with zero credentials", () => {
    const model = toWorktreePrDialogModel(
      toWorktreeExitLadder(plan({ browser_url: "https://example.test/compare" }))
    );
    expect(model.mode).toBe("browser-only");
    expect(model.rows.map(row => row.key)).toEqual(["browser"]);
    expect(model.rows[0]?.primary).toBe(true);
  });

  it("Should keep the browser row live while a blocked create states its reason", () => {
    const reason = "No branch changes to include in a PR.";
    const model = toWorktreePrDialogModel(
      toWorktreeExitLadder(
        plan({
          forge: FORGE,
          browser_url: "https://example.test/compare",
          actions: [
            {
              action: "open_pr",
              label: "Open PR",
              enabled: false,
              blocked_reason: reason,
            },
          ],
        })
      )
    );
    expect(model.blockedReason).toBe(reason);
    expect(model.rows.find(row => row.key === "create")?.blocked).toBe(true);
    expect(model.rows.find(row => row.key === "browser")?.blocked).toBeUndefined();
  });
});
