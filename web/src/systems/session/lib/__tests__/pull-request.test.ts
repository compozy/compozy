// Suite: session pull request view models
// Invariant: the chip, the card pill and the chip menu are pure projections of the daemon's PR
// facts — no branch means no chip, lookup states stay distinguishable from "no PR", stale or
// unknown facts never read Ready, unavailable actions are absent, and the agent prompts name the
// PR, its URL and head (plus up to five failing checks).
// Owning layer: unit (systems/session/lib)
import { describe, expect, it } from "vitest";

import {
  buildAddressReviewPrompt,
  buildFixChecksPrompt,
  pullRequestMenuModel,
  type PullRequestExitPlanView,
} from "../pull-request-actions";
import { pullRequestChipModel, pullRequestPill } from "../pull-request-signal";
import type {
  SessionPullRequest,
  SessionPullRequestFacts,
  SessionPullRequestLookup,
} from "../session-pull-request";

// The `_dx.md` §Session payload example: PR #932, draft, failing, review required.
const DX_PR: SessionPullRequest = {
  provider: "github",
  number: 932,
  url: "https://github.com/compozy/compozy/pull/932",
  state: "open",
  draft: true,
  head_sha: "8f3c2a1d40",
  checks: {
    state: "failing",
    passed: 12,
    failed: 2,
    pending: 0,
    failed_names: ["lint", "test-web"],
  },
  review_decision: "review_required",
  mergeable: "mergeable",
  ready_to_merge: false,
};
// The `_dx.md` `session pr -o json` example: PR #931, passing, approved, ready.
const READY_PR: SessionPullRequest = {
  provider: "github",
  number: 931,
  url: "https://github.com/compozy/compozy/pull/931",
  state: "open",
  draft: false,
  head_sha: "c1d9e07b2a",
  checks: { state: "passing", passed: 14, failed: 0, pending: 0, failed_names: [] },
  review_decision: "approved",
  mergeable: "mergeable",
  ready_to_merge: true,
};
const OK: SessionPullRequestLookup = {
  state: "ok",
  cause: "",
  observed_at: "2026-10-23T18:02:11Z",
  stale: false,
};

function facts(
  pull_request: SessionPullRequest | null,
  lookup: SessionPullRequestLookup | null = OK,
  branch = "feat/kanban"
): SessionPullRequestFacts {
  return { branch, pull_request, pull_request_lookup: lookup };
}

function prChip(input: SessionPullRequestFacts) {
  const model = pullRequestChipModel(input);
  if (model?.kind !== "pull-request") throw new Error("expected a pull-request chip");
  return model;
}

describe("pullRequestChipModel", () => {
  it("UT-060: maps the _dx.md payloads to number, state word, checks and review", () => {
    const draft = prChip(facts(DX_PR));
    expect(draft).toMatchObject({ number: 932, shape: "draft", word: "Draft", stale: false });
    expect(draft.signals.map(signal => [signal.label, signal.tone])).toEqual([
      ["2 failing", "destructive"],
      ["Review required", null],
    ]);
    expect(draft.failedNames).toEqual(["lint", "test-web"]);
    expect(draft.rows).toContainEqual({
      label: "Checks",
      value: "12 passed · 2 failed · 0 pending",
    });

    const ready = prChip(facts(READY_PR));
    expect(ready).toMatchObject({ number: 931, shape: "open", word: "Open" });
    expect(ready.signals).toEqual([
      { id: "ready", label: "Ready to merge", tone: "success", mark: "dot" },
    ]);

    const pending = prChip(
      facts({
        ...READY_PR,
        ready_to_merge: false,
        checks: { ...READY_PR.checks, state: "pending", passed: 11, pending: 3 },
      })
    );
    expect(pending.signals.map(signal => [signal.label, signal.mark])).toEqual([
      ["3 pending", "spinner"],
      ["Approved", "none"],
    ]);

    const conflicts = prChip(
      facts({ ...READY_PR, ready_to_merge: false, mergeable: "conflicting" })
    );
    expect(conflicts.signals.map(signal => [signal.label, signal.tone])).toEqual([
      ["14 passed", null],
      ["Conflicts", "destructive"],
    ]);

    const merged = prChip(facts({ ...READY_PR, state: "merged", ready_to_merge: false }));
    expect(merged).toMatchObject({ shape: "merged", word: "Merged", signals: [], rows: [] });
  });

  it("UT-061: middle-truncates a 60-character branch and keeps the full name", () => {
    const branch = `feat/${"sessions-kanban-pull-request-chip-".repeat(2)}`.slice(0, 60);
    expect(branch).toHaveLength(60);
    const model = pullRequestChipModel(facts(null, OK, branch));
    expect(model?.kind).toBe("branch");
    if (model?.kind !== "branch") return;
    expect(model.branch).toBe(branch);
    expect(model.branchLabel).toHaveLength(32);
    expect(model.branchLabel).toContain("…");
    expect(model.branchLabel.startsWith("feat/")).toBe(true);
    expect(branch.endsWith(model.branchLabel.split("…")[1]!)).toBe(true);
  });

  it("UT-062: zero checks read No checks, never a passing zero", () => {
    const model = prChip(
      facts({
        ...READY_PR,
        ready_to_merge: false,
        checks: { state: "none", passed: 0, failed: 0, pending: 0, failed_names: [] },
      })
    );
    expect(model.signals[0]).toMatchObject({ id: "checks", label: "No checks", mark: "none" });
    expect(model.rows).toContainEqual({ label: "Checks", value: "None" });
  });

  it("UT-073: branch chips keep lookup states apart from a confirmed no PR", () => {
    expect(pullRequestChipModel({ pull_request: null, pull_request_lookup: null })).toBeNull();
    expect(pullRequestChipModel(facts(null, OK, ""))).toBeNull();
    expect(
      pullRequestChipModel(
        facts(null, { state: "pending", cause: "", observed_at: null, stale: true })
      )
    ).toMatchObject({ kind: "branch", marker: "pending" });
    expect(
      pullRequestChipModel(
        facts(null, { state: "error", cause: "credential_missing", observed_at: null, stale: true })
      )
    ).toMatchObject({ kind: "branch", marker: "unavailable", detail: "GitHub not connected" });
    expect(
      pullRequestChipModel(
        facts(null, {
          state: "incomplete",
          cause: "selection_incomplete",
          observed_at: null,
          stale: true,
        })
      )
    ).toMatchObject({
      kind: "branch",
      marker: "incomplete",
      detail: "Couldn't match a PR for this branch",
    });
    expect(pullRequestChipModel(facts(null))).toMatchObject({ kind: "branch", marker: null });
  });

  it("keeps stale or failed-lookup facts, muted, and never Ready", () => {
    const stale = prChip(facts(READY_PR, { ...OK, stale: true }));
    expect(stale.stale).toBe(true);
    expect(stale.signals.map(signal => [signal.label, signal.tone])).toEqual([
      ["14 passed", null],
      ["Approved", null],
    ]);
    const failed = prChip(
      facts(DX_PR, {
        state: "error",
        cause: "rate_limited",
        observed_at: OK.observed_at,
        stale: true,
      })
    );
    expect(failed).toMatchObject({ stale: true, causeText: "Rate limited by GitHub" });
    expect(failed.signals.every(signal => signal.tone === null)).toBe(true);
    // An unknown draft flag never assembles Ready, whatever the daemon flag says.
    expect(prChip(facts({ ...READY_PR, draft: null })).signals[0]?.id).not.toBe("ready");
  });
});

describe("pullRequestPill", () => {
  it("picks the one card status: failing › changes › conflicts › ready › draft", () => {
    expect(pullRequestPill(DX_PR, OK)).toEqual({ label: "Checks failing", tone: "destructive" });
    expect(pullRequestPill({ ...READY_PR, review_decision: "changes_requested" }, OK)?.label).toBe(
      "Changes requested"
    );
    expect(pullRequestPill(READY_PR, OK)).toEqual({ label: "Ready to merge", tone: "success" });
    expect(pullRequestPill(READY_PR, { ...OK, stale: true })).toBeNull();
    expect(pullRequestPill({ ...READY_PR, state: "merged" }, OK)).toEqual({
      label: "Merged",
      tone: "info",
    });
    expect(pullRequestPill(null, OK)).toBeNull();
  });
});

describe("pullRequestMenuModel", () => {
  const exitPlan = (action: string, patch: Partial<PullRequestExitPlanView> = {}) => ({
    primary: { action, label: action === "push" ? "Push" : "Commit & push", enabled: true },
    blocked: false,
    ...patch,
  });
  const ids = (items: { id: string }[]) => items.map(item => item.id);

  it("UT-063: leads with exit, then fix, then address, then Open PR", () => {
    const changes = { ...DX_PR, review_decision: "changes_requested" as const };
    const withExit = pullRequestMenuModel({
      pullRequest: changes,
      controllable: true,
      exitPlan: exitPlan("commit_push"),
    });
    expect(withExit.primary).toMatchObject({ id: "exit", label: "Commit & push" });
    expect(ids(withExit.rest)).toEqual(["open", "refresh"]);
    expect(ids(withExit.prompts)).toEqual(["fix", "address"]);

    const failing = pullRequestMenuModel({ pullRequest: changes, controllable: true });
    expect(failing.primary?.id).toBe("fix");
    expect(ids(failing.prompts)).toEqual(["address"]);

    const reviewOnly = pullRequestMenuModel({
      pullRequest: { ...changes, checks: READY_PR.checks },
      controllable: true,
    });
    expect(reviewOnly.primary?.id).toBe("address");
    expect(reviewOnly.prompts).toEqual([]);

    const ready = pullRequestMenuModel({ pullRequest: READY_PR, controllable: true });
    expect(ready.primary).toMatchObject({ id: "open", url: READY_PR.url });
    expect(ids(ready.rest)).toEqual(["refresh"]);
    expect(ready.prompts).toEqual([]);
  });

  it("Should offer enabled commit and push when the worktree primary only commits", () => {
    const rows = [{ action: "commit_push", label: "Commit & push", enabled: true }];
    const input = {
      pullRequest: READY_PR,
      controllable: true,
      exitPlan: exitPlan("commit", { rows }),
    };
    expect(pullRequestMenuModel(input).primary).toMatchObject({
      id: "exit",
      action: "commit_push",
    });
    expect(
      pullRequestMenuModel({
        ...input,
        exitPlan: exitPlan("commit", {
          rows: [{ ...rows[0]!, enabled: false }],
        }),
      }).primary?.id
    ).toBe("open");
  });

  it("UT-063: drops a blocked exit action and keeps the plan's reason for the next primary", () => {
    const model = pullRequestMenuModel({
      pullRequest: DX_PR,
      controllable: true,
      exitPlan: exitPlan("push", {
        primary: {
          action: "push",
          label: "Push",
          enabled: false,
          blockedReason: "Branch diverged",
        },
      }),
    });
    expect(model.primary?.id).toBe("fix");
    expect(model.primaryReason).toBe("Branch diverged");
    // `view_pr` is Open PR's job, never a second exit primary.
    expect(
      pullRequestMenuModel({
        pullRequest: READY_PR,
        controllable: true,
        exitPlan: exitPlan("view_pr"),
      }).primary?.id
    ).toBe("open");
  });

  it("UT-062: a non-controllable session offers only Open PR and Refresh", () => {
    const model = pullRequestMenuModel({
      pullRequest: DX_PR,
      controllable: false,
      exitPlan: exitPlan("commit_push"),
    });
    expect(model.primary?.id).toBe("open");
    expect(ids(model.rest)).toEqual(["refresh"]);
    expect(model.prompts).toEqual([]);
  });

  it("offers Refresh without Open PR while no PR is known", () => {
    const model = pullRequestMenuModel({
      pullRequest: null,
      controllable: true,
      exitPlan: exitPlan("open_pr"),
    });
    expect(model.primary?.id).toBe("exit");
    expect(ids(model.rest)).toEqual(["refresh"]);
    expect(pullRequestMenuModel({ pullRequest: null, controllable: true }).primary).toBeNull();
  });
});

describe("agent prompts", () => {
  it("UT-064: name the PR, its URL and head, and at most five failing checks", () => {
    const fix = buildFixChecksPrompt(DX_PR);
    for (const part of ["#932", DX_PR.url, "8f3c2a1d40", "lint", "test-web"]) {
      expect(fix).toContain(part);
    }
    const many = buildFixChecksPrompt({
      ...DX_PR,
      checks: { ...DX_PR.checks, failed_names: ["a1", "b2", "c3", "d4", "e5", "f6"] },
    });
    expect(many).toContain("e5");
    expect(many).not.toContain("f6");

    const address = buildAddressReviewPrompt(DX_PR);
    expect(address).toContain("#932");
    expect(address).toContain(DX_PR.url);
  });
});
