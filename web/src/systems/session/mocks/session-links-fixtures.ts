/** What a session's linked work looks like at every scale, in the daemon's own DTOs. */
import {
  leadLink,
  linkKey,
  needsYouCount,
  type SessionLinkedWorkItem,
  type SessionWorkCandidate,
} from "../lib/session-linked-work";
import type { SessionPullRequest, SessionPullRequestLookup } from "../lib/session-pull-request";

const REPOSITORY = "github.com/compozy/compozy";

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

interface LinkOptions {
  worktree?: { name: string; removed?: boolean };
  provenance?: "created" | "user" | "agent";
  runsHere?: boolean;
  lookup?: SessionPullRequestLookup;
  /** The child session this rolled up from. */
  source?: { id: string; title: string };
}

function link(
  branch: string,
  pullRequest: SessionPullRequest | null,
  { worktree, provenance = "created", runsHere = false, lookup = OK, source }: LinkOptions
): SessionLinkedWorkItem {
  return {
    repository: REPOSITORY,
    branch,
    provenance,
    runs_here: runsHere,
    worktree_removed: worktree?.removed ?? false,
    ...(worktree ? { worktree_id: `wt-${worktree.name}`, worktree_name: worktree.name } : {}),
    ...(pullRequest ? { pr_number: pullRequest.number } : {}),
    ...(source ? { source_session_id: source.id, source_session_title: source.title } : {}),
    pull_request: pullRequest,
    pull_request_lookup: lookup,
  };
}

function pr(
  number: number,
  title: string,
  branch: string,
  patch: Partial<SessionPullRequest> = {},
  options: LinkOptions = {}
): SessionLinkedWorkItem {
  return link(
    branch,
    {
      provider: "github",
      number,
      title,
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
    options
  );
}

/** Never rolled up: a child's work reaches the parent only once it has a pull request. */
function branchOnly(
  branch: string,
  options: Omit<LinkOptions, "source"> = {}
): SessionLinkedWorkItem {
  return link(branch, null, options);
}

const worktree = (name: string, state: "ready" | "missing" = "ready") => ({
  name,
  removed: state === "missing",
});

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
  none: [] as SessionLinkedWorkItem[],
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
    pr(952, "Move the billing tables", "feat/billing-tables", {}, { provenance: "user" }),
    pr(
      950,
      "Update the migration guide",
      "docs/migration-guide",
      { review_decision: "approved" },
      { provenance: "agent" }
    ),
    HEADER_PR,
    branchOnly("spike/forge-graphql", { provenance: "user" }),
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
  // The daemon's order: urgency first, and direct before rolled-up only within one urgency.
  rolledUp: [
    pr(
      951,
      "Fix the login redirect loop",
      "fix/login-redirect",
      { checks: failing(2, ["lint", "test-web"]) },
      { source: { id: "s2", title: "Login redirect fix" } }
    ),
    pr(
      944,
      "Show the pull request in the session header",
      "feat/session-pr-chip",
      {
        draft: true,
      },
      { worktree: worktree("session-pr-chip") }
    ),
    pr(
      948,
      "Retry when GitHub is slow to answer",
      "feat/forge-retry",
      { checks: RUNNING as Checks },
      { source: { id: "s9", title: "Forge retry" } }
    ),
  ],
} satisfies Record<string, SessionLinkedWorkItem[]>;

/** An orchestrator's dozen: everything a real delivery day leaves on one session. */
export const SESSION_LINKS_DOZEN: SessionLinkedWorkItem[] = [
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
export const SESSION_LINKS_FIFTY: SessionLinkedWorkItem[] = [
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

function candidate(
  number: number,
  title: string,
  branch: string,
  state = "open"
): SessionWorkCandidate {
  return {
    repository: REPOSITORY,
    branch,
    pr_number: number,
    title,
    state,
    url: `https://github.com/compozy/compozy/pull/${number}`,
  };
}

/** What the picker's search returns: this project's pull requests, newest first. */
export const SESSION_LINK_PULL_REQUESTS: SessionWorkCandidate[] = [
  candidate(963, "Add a monthly usage report", "feat/usage-report"),
  candidate(962, "Fix the login redirect on Safari", "fix/safari-login"),
  ...SESSION_LINKS_FIFTY.flatMap(item =>
    item.pull_request
      ? [
          candidate(
            item.pull_request.number,
            item.pull_request.title ?? item.branch,
            item.branch,
            item.pull_request.state
          ),
        ]
      : []
  ).slice(0, 34),
];

/** The local branches the picker offers, including while the forge can't be reached. */
export const SESSION_LINK_BRANCHES: SessionWorkCandidate[] = [
  "feat/usage-export",
  "spike/forge-graphql",
  "docs/agent-guide",
].map(branch => ({ repository: REPOSITORY, branch }));

/** The list's identity for an item, for pinning its menu or details open in a review. */
export const sessionLinkFixtureKey = (branch: string): string => {
  const item = SESSION_LINKS_FIFTY.find(entry => entry.branch === branch);
  return item ? linkKey(item) : `${REPOSITORY}:${branch}`;
};

/** The catalog summary a list reads as. */
export function sessionLinkSummaryFixture(items: readonly SessionLinkedWorkItem[]) {
  const pullRequests = items.filter(item => item.pull_request).length;
  return {
    total: items.length,
    needs_you: needsYouCount(items),
    pull_requests: pullRequests,
    lead: leadLink(items),
  };
}

/** The item a picked candidate reads as once linked. */
export function sessionLinkFromCandidate(picked: SessionWorkCandidate): SessionLinkedWorkItem {
  const known = SESSION_LINKS_FIFTY.find(item => item.branch === picked.branch);
  if (known) return { ...known, provenance: "user" };
  return picked.pr_number
    ? pr(picked.pr_number, picked.title ?? picked.branch, picked.branch, {}, { provenance: "user" })
    : branchOnly(picked.branch, { provenance: "user" });
}
