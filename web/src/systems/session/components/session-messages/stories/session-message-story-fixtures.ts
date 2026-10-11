// Fixtures for the session message stories (S1–S4, VC-01..05). Titles, times
// and texts follow the agent-collaboration boards; runtime truth owns values.

import type { SessionMessageParty } from "../session-message-party";

export const STORY_WORKSPACE_ID = "ws_alpha";
const SENT_AT = Date.parse("2026-10-09T22:30:00.000Z");

export const at = (minutes: number) => SENT_AT + minutes * 60_000;

export const refactorBilling: SessionMessageParty = {
  sessionId: "sess-7f3a2c11d09e4b58",
  workspaceId: STORY_WORKSPACE_ID,
  title: "Refactor billing",
  agentName: "claude",
};

export const billingReviewer: SessionMessageParty = {
  sessionId: "sess-c03f9d61b2e84a07",
  workspaceId: STORY_WORKSPACE_ID,
  title: "Billing reviewer",
  agentName: "codex",
};

export const renamedSender: SessionMessageParty = {
  ...refactorBilling,
  title: "Billing refactor · PR split",
};

export const deletedSender: SessionMessageParty = { ...refactorBilling, title: null };

export const deletedTarget: SessionMessageParty = { ...billingReviewer, title: null };

export const crossWorkspaceSender: SessionMessageParty = {
  sessionId: "sess-91d0aa3c55e7b201",
  workspaceId: "ws_site",
  title: "Docs sweep",
  agentName: "gemini",
  workspaceName: "compozy-site",
};

export const QUESTION = "Is the retry budget in billing.toml per request or per job?";

export const LONG_MESSAGE = [
  "I'm splitting the billing refactor into two PRs and need you to review the boundary before I open them.",
  "PR 1 moves the HTTP client into `internal/billing/client` with no behavior change. PR 2 adds per-request retries on top of the existing per-job budget.",
  "Questions: does anything outside `internal/billing` import the client directly? Is it safe to keep the per-job budget as the outer cap? Should the per-request cap live in `billing.toml` or in code?",
  "Context: the retry middleware lives in `internal/billing/retry.go`, and the budget is decremented once per attempt on a shared counter keyed by job id.",
  "I'll hold both PRs until you answer. If anything in the split looks wrong, say so and I'll restructure before opening them.",
].join("\n\n");

export const ANSWER =
  "Per job. `retry_budget` in `billing.toml` caps total attempts across all of a job's requests; `retry.go:41` decrements one shared counter per job id. Per-request limits don't exist today.";
