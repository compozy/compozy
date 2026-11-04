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
import { formatMessageTimestamp } from "../format-timestamp";
import { pullRequestChipModel, pullRequestPill } from "../pull-request-signal";
import {
  parseSessionLinkInput,
  repositoryForgeName,
  sessionLinkExactMatch,
  sessionLinkSearch,
} from "../session-link-input";
import {
  groupLinks,
  leadLink,
  linkForkTitle,
  linkStatus,
  linkOriginLabel,
  linkSourceLabel,
  linkTitle,
  linkUnlinkedNote,
  linkedCandidateKeys,
  candidateIsLinked,
  linkCount,
  linkedCountLabel,
  summaryLinkCount,
  needsYouCount,
  summaryMoreLabel,
  type SessionLinkedWorkItem,
} from "../session-linked-work";
import type {
  SessionPullRequest,
  SessionPullRequestFacts,
  SessionPullRequestLookup,
} from "../session-pull-request";

import {
  SESSION_LINK_BRANCHES,
  SESSION_LINK_PULL_REQUESTS,
  SESSION_LINK_SETS,
  SESSION_LINKS_DOZEN,
  SESSION_LINKS_FIFTY,
} from "../../mocks/session-links-fixtures";

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
    expect(draft.lines).toContainEqual({
      id: "checks",
      text: "2 of 14 checks failing",
      tone: "destructive",
      mark: "dot",
    });
    expect(prChip(facts({ ...DX_PR, checks: { ...DX_PR.checks, failed: 7 } })).failedMore).toBe(5);

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
    expect(merged).toMatchObject({ shape: "merged", word: "Merged", signals: [], lines: [] });
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
    expect(model.lines).toContainEqual({
      id: "checks",
      text: "No checks",
      tone: null,
      mark: "dot",
    });
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
    const retryAfter = "2026-11-04T16:05:00Z";
    const limited = prChip(
      facts(DX_PR, {
        state: "error",
        cause: "rate_limited",
        observed_at: OK.observed_at,
        stale: true,
        retry_after: retryAfter,
      })
    );
    expect(limited.causeText).toBe(
      `Rate limited by GitHub until ${formatMessageTimestamp(Date.parse(retryAfter))}`
    );
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

// Invariant: linked work reads in the daemon's order, grouped but never re-sorted, so the header,
// the list and the catalog's cards name one lead; a summary never claims more than its counts say.
describe("session linked work", () => {
  const titles = (items: readonly SessionLinkedWorkItem[]) => items.map(linkTitle);

  it("Should put what needs you first and keep the terminal items in Done", () => {
    const groups = groupLinks(SESSION_LINKS_DOZEN);
    expect(groups.map(group => group.id)).toEqual(["needs-you", "in-progress", "done"]);
    expect(groups[0]?.items.map(item => item.pr_number)).toEqual([951, 957]);
    expect(groups[2]?.items.every(item => item.pull_request?.state !== "open")).toBe(true);
    expect(needsYouCount(SESSION_LINKS_DOZEN)).toBe(2);
    expect(leadLink(SESSION_LINKS_DOZEN)?.pr_number).toBe(951);
  });

  it("Should hold fifty items in one pass over every group", () => {
    const shown = groupLinks(SESSION_LINKS_FIFTY).flatMap(group => group.items);
    expect(shown).toHaveLength(50);
  });

  it("Should keep the daemon's order inside a group and lead with its first item", () => {
    const [draft, ready] = [SESSION_LINK_SETS.onePr[0]!, SESSION_LINK_SETS.worktreeRemoved[0]!];
    const child = { ...SESSION_LINK_SETS.rolledUp[2]!, pull_request: ready.pull_request };
    const runsHere = { ...SESSION_LINK_SETS.runsHere[0]!, branch: "feat/runs-here" };
    const served = [child, draft, ready, runsHere];
    expect(groupLinks(served).flatMap(group => group.items)).toEqual(served);
    expect(leadLink(served)).toBe(child);
  });

  it("Should name the same lead as the daemon's catalog summary", () => {
    const served = SESSION_LINK_SETS.rolledUp;
    const summary = { total: 3, needs_you: 1, pull_requests: 3, lead: served[0] };
    expect(leadLink(served)).toBe(summary.lead);
    expect(groupLinks(served)[0]?.items[0]).toBe(summary.lead);
  });

  it("Should name the rolled-up source as a fact and say nothing about who added it", () => {
    const rolled = SESSION_LINK_SETS.rolledUp[0]!;
    expect(linkOriginLabel(rolled)).toBe("From Login redirect fix");
    expect(linkSourceLabel({ ...rolled, provenance: "agent" }, "Claude")).toBeNull();
    expect(linkOriginLabel(SESSION_LINK_SETS.rolledUp[1]!)).toBeNull();
  });

  it("Should name every refused fork in one sentence", () => {
    expect(linkForkTitle([212])).toBe("#212 comes from a fork");
    expect(linkForkTitle([212, 214])).toBe("#212, #214 come from forks");
  });

  it("Should say who added an item only for the exceptions", () => {
    const [user, agent, created] = SESSION_LINK_SETS.addedBy;
    expect(linkSourceLabel(user!, "Claude")).toBe("Added by you");
    expect(linkSourceLabel(agent!, "Claude")).toBe("Added by Claude");
    expect(linkSourceLabel(created!, "Claude")).toBeNull();
  });

  it.each([
    ["all pull requests", SESSION_LINK_SETS.mixed, "4 linked PRs"],
    ["two pull requests", SESSION_LINK_SETS.runsHere, "2 linked PRs"],
    ["a branch among them", SESSION_LINK_SETS.addedBy, "4 linked"],
  ])("Should count the whole list with %s", (_name, items, label) => {
    expect(linkedCountLabel(linkCount(items))).toBe(label);
  });

  it("Should read the same count from the catalog summary", () => {
    const summary = { total: 4, needs_you: 1, pull_requests: 3, lead: READY_ITEM() };
    expect(summaryLinkCount(summary)).toEqual({ total: 4, allPullRequests: false, needsYou: 1 });
    expect(linkCount(SESSION_LINK_SETS.mixed)).toEqual({
      total: 4,
      allPullRequests: true,
      needsYou: 1,
    });
  });

  it.each([
    [{ total: 4, needs_you: 1, pull_requests: 4, lead: READY_ITEM() }, "+3 PRs"],
    [{ total: 4, needs_you: 0, pull_requests: 3, lead: READY_ITEM() }, "+3 more"],
    [{ total: 1, needs_you: 0, pull_requests: 1, lead: READY_ITEM() }, null],
    [{ total: 4, needs_you: 0, pull_requests: 3, lead: BRANCH_ITEM() }, "+3 PRs"],
  ])("Should read the same label from the catalog summary", (summary, label) => {
    expect(summaryMoreLabel(summary)).toBe(label);
  });

  it("Should keep status words honest when facts are missing", () => {
    const [failedLookup, stale] = SESSION_LINK_SETS.lookupFailed;
    expect(linkStatus(failedLookup!)).toEqual({ label: "PR status unknown", tone: "neutral" });
    expect(linkStatus(stale!).tone).toBe("neutral");
    expect(linkStatus(SESSION_LINK_SETS.worktreeNoPr[0]!).label).toBe("No pull request yet");
    expect(linkStatus({ ...failedLookup!, pull_request_lookup: null }).label).toBe(
      "No pull request yet"
    );
  });

  it("Should fall back to the branch when the forge sends no title", () => {
    const item = SESSION_LINK_SETS.onePr[0]!;
    const untitled = { ...item, pull_request: { ...item.pull_request!, title: undefined } };
    expect(linkTitle(untitled)).toBe(item.branch);
    expect(titles(SESSION_LINK_SETS.worktreeNoPr)).toEqual(["session-links"]);
  });

  it("Should reassure in the words of the forge that owns the pull request", () => {
    expect(linkUnlinkedNote(SESSION_LINK_SETS.onePr[0]!)).toContain("still on GitHub");
    expect(repositoryForgeName("git.acme.dev/team/app")).toBe("git.acme.dev");
    expect(linkUnlinkedNote(SESSION_LINK_SETS.worktreeNoPr[0]!)).toContain("files are untouched");
  });

  it("Should mark direct items as linked and leave rolled-up ones pickable", () => {
    const linked = linkedCandidateKeys(SESSION_LINK_SETS.rolledUp);
    const pick = (number: number) =>
      SESSION_LINK_PULL_REQUESTS.find(candidate => candidate.pr_number === number)!;
    expect(candidateIsLinked(pick(944), linked)).toBe(true);
    expect(candidateIsLinked(pick(951), linked)).toBe(false);
    // A newer pull request on a reused branch is a different pick.
    expect(candidateIsLinked({ ...pick(944), pr_number: 9944 }, linked)).toBe(false);
  });

  describe("link input", () => {
    it("Should read a nested project path from a pasted link", () => {
      expect(
        parseSessionLinkInput("https://gitlab.com/group/subgroup/repo/-/merge_requests/12")
      ).toEqual({ kind: "link", repository: "gitlab.com/group/subgroup/repo", number: 12 });
    });

    const search = {
      repository: "github.com/compozy/compozy",
      pullRequests: SESSION_LINK_PULL_REQUESTS,
      branches: SESSION_LINK_BRANCHES,
      searching: false,
      refused: false,
    };

    it.each([
      ["https://github.com/acme/app/pull/12", "github.com/acme/app", 12],
      ["https://gitlab.com/acme/app/-/merge_requests/12", "gitlab.com/acme/app", 12],
      ["https://bitbucket.org/acme/app/pull-requests/12/title", "bitbucket.org/acme/app", 12],
    ])("Should read %s as one pull request", (url, repository, number) => {
      expect(parseSessionLinkInput(url)).toEqual({ kind: "link", repository, number });
    });

    it("Should read a number and leave words alone", () => {
      expect(parseSessionLinkInput(" #42 ")).toEqual({ kind: "number", number: 42 });
      expect(parseSessionLinkInput("login fix")).toEqual({ kind: "text", text: "login fix" });
    });

    it("Should refuse a link from another repository before searching for it", () => {
      expect(
        sessionLinkSearch({ ...search, query: "https://github.com/acme/billing/pull/212" })
      ).toEqual({ kind: "other-project", repository: "github.com/acme/billing", number: 212 });
    });

    it("Should only tick a pasted link's pull request in that link's repository", () => {
      const same = "https://github.com/COMPOZY/compozy/pull/963";
      expect(sessionLinkExactMatch(same, SESSION_LINK_PULL_REQUESTS)?.pr_number).toBe(963);
      expect(
        sessionLinkExactMatch(
          "https://github.com/acme/billing/pull/963",
          SESSION_LINK_PULL_REQUESTS
        )
      ).toBeNull();
      expect(sessionLinkExactMatch("#963", SESSION_LINK_PULL_REQUESTS)?.pr_number).toBe(963);
    });

    it("Should treat the daemon's refusal as another project's", () => {
      expect(
        sessionLinkSearch({
          ...search,
          repository: null,
          refused: true,
          query: "https://github.com/acme/billing/pull/212",
        }).kind
      ).toBe("other-project");
    });

    it("Should say it can't find a number only once the read has answered", () => {
      const empty = { ...search, pullRequests: [], branches: [] };
      expect(sessionLinkSearch({ ...empty, query: "#99999" })).toEqual({
        kind: "unknown",
        query: "#99999",
      });
      expect(sessionLinkSearch({ ...empty, searching: true, query: "#99999" }).kind).toBe(
        "results"
      );
    });

    it("Should narrow a number to that pull request alone", () => {
      const result = sessionLinkSearch({ ...search, query: "#963" });
      expect(result).toMatchObject({ kind: "results", branches: [] });
      expect(result.kind === "results" ? result.pullRequests : []).toHaveLength(1);
    });

    it("Should drop a branch whose pull request is already on screen", () => {
      const branch = { repository: search.repository, branch: "feat/usage-report" };
      const result = sessionLinkSearch({ ...search, query: "usage", branches: [branch] });
      expect(result.kind === "results" ? result.branches : null).toEqual([]);
    });
  });
});

function READY_ITEM(): SessionLinkedWorkItem {
  return SESSION_LINK_SETS.onePr[0]!;
}

function BRANCH_ITEM(): SessionLinkedWorkItem {
  return SESSION_LINK_SETS.worktreeNoPr[0]!;
}
