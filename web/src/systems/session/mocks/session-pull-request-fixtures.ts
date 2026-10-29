/**
 * Branch and pull request facts for every `SessionPullRequestChip` state (board §S1 VC-02…VC-14),
 * shared by the chip's stories and its gallery prototype.
 */
import type {
  SessionPullRequest,
  SessionPullRequestFacts,
  SessionPullRequestLookup,
} from "../lib/session-pull-request";

const OK: SessionPullRequestLookup = {
  state: "ok",
  cause: "",
  observed_at: "2026-10-23T18:01:40Z",
  stale: false,
};
const OPEN: SessionPullRequest = {
  provider: "github",
  number: 931,
  url: "https://github.com/compozy/compozy/pull/931",
  state: "open",
  draft: false,
  head_sha: "c1d9e07b2a",
  checks: { state: "passing", passed: 14, failed: 0, pending: 0, failed_names: [] },
  review_decision: "approved",
  mergeable: "mergeable",
  ready_to_merge: false,
};
const BRANCH = "feat/session-pr-chip";

const branch = (lookup: SessionPullRequestLookup): SessionPullRequestFacts => ({
  branch: BRANCH,
  pull_request: null,
  pull_request_lookup: lookup,
});
export const pullRequestFacts = (
  patch: Partial<SessionPullRequest>,
  lookup: SessionPullRequestLookup = OK
): SessionPullRequestFacts => ({
  branch: BRANCH,
  pull_request: { ...OPEN, ...patch },
  pull_request_lookup: lookup,
});

/** Board §S1 VC-02…VC-14, in board order; `id` is the prototype state. */
export const SESSION_PULL_REQUEST_CHIP_STATES: {
  id: string;
  vc: string;
  label: string;
  facts: SessionPullRequestFacts;
  refreshing?: boolean;
}[] = [
  {
    id: "lookup-pending",
    vc: "VC-02",
    label: "Lookup pending",
    facts: branch({ ...OK, state: "pending", stale: true }),
  },
  { id: "branch", vc: "VC-03", label: "Branch, no PR", facts: branch(OK) },
  {
    id: "unavailable",
    vc: "VC-04",
    label: "Unavailable",
    facts: branch({ state: "error", cause: "credential_missing", observed_at: null, stale: true }),
  },
  {
    id: "incomplete",
    vc: "VC-05",
    label: "Incomplete",
    facts: branch({
      state: "incomplete",
      cause: "selection_incomplete",
      observed_at: null,
      stale: true,
    }),
  },
  {
    id: "checks-pending",
    vc: "VC-06",
    label: "Checks pending",
    facts: pullRequestFacts({
      checks: { state: "pending", passed: 11, failed: 0, pending: 3, failed_names: [] },
      review_decision: "review_required",
    }),
  },
  {
    id: "checks-failing",
    vc: "VC-07",
    label: "Checks failing",
    facts: pullRequestFacts({
      number: 932,
      checks: {
        state: "failing",
        passed: 12,
        failed: 2,
        pending: 0,
        failed_names: ["lint", "test-web"],
      },
      review_decision: "review_required",
    }),
  },
  {
    id: "changes-requested",
    vc: "VC-08",
    label: "Changes requested",
    facts: pullRequestFacts({ review_decision: "changes_requested" }),
  },
  {
    id: "draft",
    vc: "VC-09",
    label: "Draft",
    facts: pullRequestFacts({ draft: true, review_decision: "review_required" }),
  },
  {
    id: "ready-to-merge",
    vc: "VC-10",
    label: "Ready to merge",
    facts: pullRequestFacts({ ready_to_merge: true }),
  },
  {
    id: "conflicts",
    vc: "VC-11",
    label: "Conflicts",
    facts: pullRequestFacts({ mergeable: "conflicting" }),
  },
  { id: "merged", vc: "VC-12", label: "Merged", facts: pullRequestFacts({ state: "merged" }) },
  { id: "closed", vc: "VC-12", label: "Closed", facts: pullRequestFacts({ state: "closed" }) },
  {
    id: "no-checks",
    vc: "VC-06",
    label: "No checks",
    facts: pullRequestFacts({
      checks: { state: "none", passed: 0, failed: 0, pending: 0, failed_names: [] },
    }),
  },
  {
    id: "stale",
    vc: "VC-13",
    label: "Stale facts",
    facts: pullRequestFacts(
      { ready_to_merge: true },
      { ...OK, state: "error", cause: "rate_limited", stale: true }
    ),
  },
  {
    id: "refreshing",
    vc: "VC-14",
    label: "Refreshing",
    facts: pullRequestFacts({}),
    refreshing: true,
  },
];
