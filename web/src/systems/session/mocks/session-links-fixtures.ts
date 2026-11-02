/**
 * What a session's linked work looks like at every scale, for the `session-links-*` drafts.
 * A title on a pull request is a new fact: the forge status read does not return one yet.
 */
import type {
  SessionPullRequest,
  SessionPullRequestFacts,
  SessionPullRequestLookup,
} from "../lib/session-pull-request";

/** `created`: the session made it. `you`: the user linked it. `agent`: the agent linked it. */
export type SessionLinkSource = "created" | "you" | "agent";

export interface SessionLink {
  id: string;
  /** The pull request's title; `null` for a branch or worktree with no pull request. */
  title: string | null;
  facts: SessionPullRequestFacts;
  worktree?: { name: string; state: "ready" | "missing" };
  source: SessionLinkSource;
  /** The agent's process runs in this worktree. Only one link at most. */
  runsHere?: boolean;
}

export type SessionRunsIn = { kind: "project" | "worktree"; name: string };

const OBSERVED = "2026-11-02T17:40:00Z";
const OK: SessionPullRequestLookup = {
  state: "ok",
  cause: "",
  observed_at: OBSERVED,
  stale: false,
};
const PASSING = { state: "passing", passed: 14, failed: 0, pending: 0, failed_names: [] };
const RUNNING = { state: "pending", passed: 9, failed: 0, pending: 5, failed_names: [] };

type Checks = SessionPullRequest["checks"];
const failing = (failed: number, names: string[]): Checks => ({
  state: "failing",
  passed: 12,
  failed,
  pending: 0,
  failed_names: names,
});

interface PrOptions {
  worktree?: SessionLink["worktree"];
  source?: SessionLinkSource;
  runsHere?: boolean;
  lookup?: SessionPullRequestLookup;
}

function pr(
  number: number,
  title: string,
  branch: string,
  patch: Partial<SessionPullRequest> = {},
  options: PrOptions = {}
): SessionLink {
  const { lookup = OK, source = "created", ...rest } = options;
  return {
    id: `pr-${number}`,
    title,
    source,
    ...rest,
    facts: {
      branch,
      pull_request_lookup: lookup,
      pull_request: {
        provider: "github",
        number,
        url: `https://github.com/compozy/compozy/pull/${number}`,
        state: "open",
        draft: false,
        head_sha: "c1d9e07b2a",
        checks: PASSING as Checks,
        review_decision: "review_required",
        mergeable: "mergeable",
        ready_to_merge: false,
        ...patch,
      },
    },
  };
}

function branchOnly(
  branch: string,
  options: PrOptions & { lookup?: SessionPullRequestLookup } = {}
): SessionLink {
  const { lookup = OK, source = "created", ...rest } = options;
  return {
    id: `branch-${branch}`,
    title: null,
    source,
    ...rest,
    facts: { branch, pull_request: null, pull_request_lookup: lookup },
  };
}

const worktree = (name: string, state: "ready" | "missing" = "ready") => ({ name, state });

const HEADER_PR = pr(
  944,
  "Show the pull request in the session header",
  "feat/session-pr-chip",
  { draft: true },
  { worktree: worktree("session-pr-chip") }
);

const FAILING_PR = pr(
  951,
  "Fix the login redirect loop",
  "fix/login-redirect",
  { checks: failing(2, ["lint", "test-web"]) },
  { worktree: worktree("login-redirect") }
);
const REVIEW_PR = pr(
  948,
  "Retry when GitHub is slow to answer",
  "feat/forge-retry",
  { checks: RUNNING as Checks },
  { worktree: worktree("forge-retry") }
);
const MERGED_PR = pr(
  939,
  "Drop the old session cache",
  "refactor/session-cache",
  { state: "merged" },
  { worktree: worktree("session-cache", "missing") }
);
const CLOSED_PR = pr(936, "Try a second layout for the chip", "proto/chip-layout", {
  state: "closed",
});
const NO_PR = branchOnly("proto/session-links", { worktree: worktree("session-links") });

export const SESSION_LINK_SETS = {
  none: [] as SessionLink[],
  onePr: [HEADER_PR],
  mixed: [FAILING_PR, REVIEW_PR, MERGED_PR, CLOSED_PR],
  worktreeNoPr: [NO_PR],
  worktreeRemoved: [
    pr(
      931,
      "Add the session environment menu",
      "feat/environment-menu",
      { ready_to_merge: true, review_decision: "approved" },
      { worktree: worktree("environment-menu", "missing") }
    ),
    NO_PR,
  ],
  lookupFailed: [
    branchOnly("fix/attachment-rebase", {
      worktree: worktree("attachment-rebase"),
      lookup: { state: "error", cause: "credential_missing", observed_at: null, stale: true },
    }),
    pr(
      942,
      "Keep the last answer when GitHub is down",
      "fix/forge-cache",
      {},
      {
        lookup: { ...OK, state: "error", cause: "rate_limited", stale: true },
      }
    ),
  ],
  addedBy: [
    pr(952, "Move the billing tables", "feat/billing-tables", {}, { source: "you" }),
    pr(
      950,
      "Update the migration guide",
      "docs/migration-guide",
      { review_decision: "approved" },
      { source: "agent" }
    ),
    HEADER_PR,
    branchOnly("spike/forge-graphql", { source: "you" }),
  ],
  runsHere: [
    pr(
      944,
      "Show the pull request in the session header",
      "feat/session-pr-chip",
      { draft: true },
      { worktree: worktree("session-pr-chip"), runsHere: true }
    ),
    REVIEW_PR,
  ],
} satisfies Record<string, SessionLink[]>;

/** An orchestrator's dozen: everything a real delivery day leaves on one session. */
export const SESSION_LINKS_DOZEN: SessionLink[] = [
  FAILING_PR,
  pr(
    957,
    "Let agents read their own usage",
    "feat/agent-usage",
    { review_decision: "changes_requested" },
    { worktree: worktree("agent-usage") }
  ),
  pr(
    955,
    "Speed up the session list",
    "perf/session-list",
    { ready_to_merge: true, review_decision: "approved" },
    { worktree: worktree("session-list") }
  ),
  REVIEW_PR,
  pr(
    947,
    "Add Later to the board",
    "feat/board-later",
    { review_decision: "approved" },
    {
      worktree: worktree("board-later"),
    }
  ),
  HEADER_PR,
  pr(
    945,
    "Remember the sidebar width",
    "fix/sidebar-width",
    {},
    { worktree: worktree("sidebar-width", "missing") }
  ),
  NO_PR,
  pr(941, "Show who started a session", "feat/session-creator", { state: "merged" }),
  MERGED_PR,
  pr(938, "Rename the Tasks window", "refactor/tasks-window", { state: "merged" }),
  CLOSED_PR,
];

const FIFTY_TITLES = [
  "Split the billing tables",
  "Retry webhook deliveries",
  "Fix the empty project list",
  "Add a filter to the Automations list",
  "Clean up the old terminal tabs",
  "Teach the board about Later",
  "Stop the dock badge from flickering",
  "Show model cost per session",
  "Sort projects by last activity",
  "Keep drafts when a session restarts",
  "Add a shortcut for the side panel",
  "Explain why a tool call was blocked",
];

const FIFTY_PATCHES: Partial<SessionPullRequest>[] = [
  { review_decision: "approved" },
  { state: "merged" },
  { draft: true },
  { state: "merged" },
  { checks: RUNNING as Checks },
  { state: "merged" },
  { review_decision: "approved", ready_to_merge: true },
  { state: "closed" },
];

/** Fifty links: the size the list has to survive, with a real mix of states. */
export const SESSION_LINKS_FIFTY: SessionLink[] = [
  ...SESSION_LINKS_DOZEN,
  ...Array.from({ length: 38 }, (_, index) => {
    const number = 900 - index;
    const title = FIFTY_TITLES[index % FIFTY_TITLES.length]!;
    const slug = title.toLowerCase().replaceAll(" ", "-");
    return pr(number, title, `feat/${slug}-${index + 1}`, {
      ...FIFTY_PATCHES[index % FIFTY_PATCHES.length],
    });
  }),
];

/** What the picker offers: this project's pull requests and branches, newest first. */
export const SESSION_LINK_CANDIDATES: SessionLink[] = [
  pr(963, "Add a monthly usage report", "feat/usage-report", {}, { source: "you" }),
  pr(962, "Fix the login redirect on Safari", "fix/safari-login", {}, { source: "you" }),
  ...SESSION_LINKS_FIFTY.filter(link => link.title !== null).slice(0, 34),
  ...["feat/usage-export", "spike/forge-graphql", "docs/agent-guide"].map(branch =>
    branchOnly(branch, { source: "you" })
  ),
];
