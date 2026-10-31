/** Board sessions shared by the Kanban stories and gallery prototypes. */
import type { SessionBoardColumnId, SessionPullRequest } from "../lib/session-board-contract";
import { primarySessionFixture, sessionRuntime } from "./fixtures";
import type { SessionPayload } from "../types";

const NOW = Date.now();
const ago = (minutes: number) => new Date(NOW - minutes * 60_000).toISOString();
const COORDINATOR = "Sessions Kanban — coordinator";

function pr(number: number, overrides: Partial<SessionPullRequest> = {}): SessionPullRequest {
  return {
    provider: "github",
    number,
    url: `https://github.com/compozy/compozy/pull/${number}`,
    state: "open",
    draft: false,
    head_sha: "8f3c2a1d40",
    checks: { state: "passing", passed: 12, failed: 0, pending: 0, failed_names: [] },
    review_decision: "review_required",
    mergeable: "mergeable",
    ready_to_merge: false,
    ...overrides,
  };
}

function boardSession(
  id: string,
  column: SessionBoardColumnId,
  overrides: Partial<SessionPayload> & { provider?: string; minutes?: number } = {}
): SessionPayload {
  const { provider = "claude", minutes = 5, ...rest } = overrides;
  const pullRequest = rest.pull_request ?? null;
  return {
    ...primarySessionFixture,
    id,
    agent_name: provider,
    runtime: sessionRuntime(provider),
    state: "active",
    badge: "idle",
    board_column: column,
    archived_at: null,
    updated_at: ago(minutes),
    pull_request: pullRequest,
    pull_request_lookup: rest.branch
      ? { state: "ok", cause: "", observed_at: ago(1), stale: false }
      : null,
    ...rest,
  };
}

const FAILING = { state: "failing", passed: 9, failed: 2, pending: 0, failed_names: ["lint"] };

export const SESSIONS_BOARD_SESSIONS: Record<SessionBoardColumnId, SessionPayload[]> = {
  needs_you: [
    boardSession("s2", "needs_you", {
      name: "Which token scope does the GraphQL provider need?",
      badge: "waiting-for-input",
      branch: "feat/github-graphql",
      pull_request: pr(943, { checks: FAILING as SessionPullRequest["checks"] }),
      parent_title: COORDINATOR,
      provider: "codex",
      minutes: 6,
    }),
    boardSession("s10", "needs_you", {
      name: "Rebase attachment lifecycle onto main",
      badge: "failed",
      branch: "fix/attachment-rebase",
      provider: "codex",
      minutes: 18,
    }),
  ],
  working: [
    boardSession("s4", "working", {
      name: "Topbar PR chip — web slice",
      badge: "running",
      branch: "feat/session-pr-chip",
      pull_request: pr(944, { draft: true }),
      parent_title: COORDINATOR,
      minutes: 0,
    }),
  ],
  waiting: [
    boardSession("s1", "waiting", {
      name: COORDINATOR,
      badge: "waiting",
      waiting_on: "sessions",
      agent_name: "orchestrator",
      children: { total: 4, needs_you: 1, working: 1, waiting: 0, in_review: 1, done: 1 },
      minutes: 4,
    }),
    boardSession("s19", "waiting", {
      name: "Merge train: #919–#923",
      badge: "waiting",
      waiting_on: "monitor",
      minutes: 7,
    }),
    boardSession("s20", "waiting", {
      name: "Watch CI on the board column PR",
      badge: "waiting",
      waiting_on: "monitor",
      branch: "feat/board-waiting-column",
      pull_request: pr(951, {
        checks: { state: "pending", passed: 8, failed: 0, pending: 4, failed_names: [] },
      }),
      minutes: 2,
    }),
  ],
  in_review: [
    boardSession("s9", "in_review", {
      name: "Board column SQL expression + equivalence test",
      badge: "done",
      branch: "feat/board-column-sql",
      pull_request: pr(945, { review_decision: "approved", ready_to_merge: true }),
      parent_title: COORDINATOR,
      provider: "codex",
      minutes: 22,
    }),
    boardSession("s7", "in_review", {
      name: "Forge status optional fields",
      branch: "feat/forge-status-fields",
      pull_request: pr(942, { review_decision: "changes_requested" }),
      minutes: 60,
    }),
    boardSession("s12", "in_review", {
      name: "Archive bar disclosure",
      badge: "done",
      branch: "feat/board-archive-bar",
      pull_request: pr(948, { mergeable: "conflicting" }),
      provider: "codex",
      minutes: 300,
    }),
    boardSession("s13", "in_review", {
      name: "Hide child sessions filter",
      branch: "feat/children-filter",
      pull_request: pr(950),
      minutes: 360,
    }),
  ],
  done: [
    boardSession("s14", "done", {
      name: "Retain unsent composer attachments across sessions",
      branch: "fix/composer-attachments",
      pull_request: pr(928, { state: "merged" }),
      minutes: 1440,
    }),
    boardSession("s17", "done", {
      name: "Explore Synara kanban model",
      branch: "spike/synara-kanban",
      pull_request: pr(930, { state: "closed" }),
      provider: "codex",
      minutes: 4320,
    }),
    boardSession("s18", "done", { name: "", minutes: 5760 }),
  ],
};

export const SESSIONS_BOARD_ARCHIVED: SessionPayload[] = [
  boardSession("a1", "done", {
    name: "Tasks removal — slice 3",
    archived_at: ago(2880),
    branch: "spike/tasks-removal",
    pull_request: pr(901, { state: "closed" }),
    provider: "codex",
  }),
  boardSession("a3", "done", {
    name: "Provider usage collection",
    archived_at: ago(2880),
    branch: "feat/provider-usage",
    pull_request: pr(871, { state: "merged" }),
  }),
];

const ahead = (minutes: number) => new Date(NOW + minutes * 60_000).toISOString();

/** Sessions parked in Later: a reason with a reminder, one with waiting agent messages, a bare one. */
export const SESSIONS_BOARD_LATER: SessionPayload[] = [
  boardSession("l1", "done", {
    name: "Migrate billing webhooks to v2",
    state: "stopped",
    branch: "feat/billing-webhooks-v2",
    later: {
      parked_at: ago(180),
      reason: "Waiting on Stripe to enable the v2 endpoint",
      return_at: ahead(3 * 24 * 60),
      pending_messages: 0,
    },
    minutes: 180,
  }),
  boardSession("l2", "done", {
    name: "Design review notes for onboarding",
    state: "stopped",
    provider: "codex",
    later: {
      parked_at: ago(2 * 24 * 60),
      reason: "After the design crit on Thursday",
      return_at: null,
      pending_messages: 2,
    },
    minutes: 2 * 24 * 60,
  }),
  boardSession("l3", "done", {
    name: "Spike: local embeddings cache",
    state: "stopped",
    later: { parked_at: ago(6 * 24 * 60), return_at: null, pending_messages: 0 },
    minutes: 6 * 24 * 60,
  }),
];

/** A session the daemon brought back from Later on its reminder date: it sits in Needs you. */
export const SESSIONS_BOARD_BACK_FROM_LATER: SessionPayload = boardSession("s30", "needs_you", {
  name: "Renew the Apple developer certificate",
  state: "stopped",
  badge: "needs-attention",
  returned_from_later_at: ago(12),
  minutes: 12,
});
