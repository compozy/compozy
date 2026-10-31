// Suite: session-board-card
// Invariant: a board card shows at most one status — the needs-you reason in
// Needs you, else the most actionable PR signal — and "Ready to merge" only
// when the daemon reports it on current facts; the meta row names the PR, the
// branch, or (All projects without a PR) the workspace; the child rollup counts
// the daemon's per-column children and flags a child that needs you; the
// lineage line names a child's parent but a fork's or continuation's source.
// Owning layer: unit (systems/session/lib). Rendering lives in sessions-board.test.tsx.
import { describe, expect, it } from "vitest";

import { primarySessionFixture } from "../../testing";
import type { SessionPayload } from "../../types";
import {
  sessionBoardCardLineage,
  sessionBoardCardMeta,
  sessionBoardCardRollup,
  sessionBoardCardStatus,
  sessionBoardCardWaiting,
} from "../session-board-card";
import type { SessionPullRequest } from "../session-board-contract";

function pr(overrides: Partial<SessionPullRequest> = {}): SessionPullRequest {
  return {
    provider: "github",
    number: 931,
    url: "https://github.com/compozy/compozy/pull/931",
    state: "open",
    draft: false,
    head_sha: "c1d9e07b2a",
    checks: { state: "passing", passed: 14, failed: 0, pending: 0, failed_names: [] },
    review_decision: "review_required",
    mergeable: "mergeable",
    ready_to_merge: false,
    ...overrides,
  };
}

const OK_LOOKUP: NonNullable<SessionPayload["pull_request_lookup"]> = {
  state: "ok",
  cause: "",
  observed_at: "2026-10-23T18:02:11Z",
  stale: false,
};

function session(overrides: Partial<SessionPayload> = {}): SessionPayload {
  return {
    ...primarySessionFixture,
    badge: "idle",
    board_column: "in_review",
    branch: "feat/kanban",
    pull_request: pr(),
    pull_request_lookup: OK_LOOKUP,
    ...overrides,
  };
}

describe("sessionBoardCardStatus (UT-068)", () => {
  it("Should name the needs-you reason before any PR signal", () => {
    const failing = pr({ checks: { ...pr().checks, state: "failing", failed: 2 } });
    expect(
      sessionBoardCardStatus(
        session({ badge: "waiting-for-auth", board_column: "needs_you", pull_request: failing })
      )
    ).toEqual({ label: "Auth", tone: "accent" });
    expect(sessionBoardCardStatus(session({ badge: "failed", board_column: "needs_you" }))).toEqual(
      { label: "Failed", tone: "danger" }
    );
    expect(
      sessionBoardCardStatus(session({ badge: "waiting-for-input", board_column: "needs_you" }))
    ).toEqual({ label: "Input", tone: "accent" });
  });

  it("Should map the PR signal by precedence", () => {
    expect(
      sessionBoardCardStatus(
        session({ pull_request: pr({ review_decision: "changes_requested" }) })
      )
    ).toEqual({ label: "Changes requested", tone: "danger" });
    expect(
      sessionBoardCardStatus(
        session({
          pull_request: pr({
            checks: { ...pr().checks, state: "failing", failed: 1 },
            review_decision: "changes_requested",
            mergeable: "conflicting",
          }),
        })
      )?.label
    ).toBe("Checks failing");
    expect(
      sessionBoardCardStatus(session({ pull_request: pr({ mergeable: "conflicting" }) }))?.label
    ).toBe("Conflicts");
    expect(sessionBoardCardStatus(session({ pull_request: pr({ draft: true }) }))).toEqual({
      label: "Draft",
      tone: "neutral",
    });
    expect(
      sessionBoardCardStatus(
        session({ board_column: "done", pull_request: pr({ state: "merged" }) })
      )
    ).toEqual({ label: "Merged", tone: "info" });
    expect(
      sessionBoardCardStatus(
        session({ board_column: "done", pull_request: pr({ state: "closed" }) })
      )?.label
    ).toBe("Closed");
  });

  it("Should show Ready to merge only on current facts the daemon marked ready", () => {
    const ready = pr({ review_decision: "approved", ready_to_merge: true });
    expect(sessionBoardCardStatus(session({ pull_request: ready }))).toEqual({
      label: "Ready to merge",
      tone: "success",
    });
    expect(
      sessionBoardCardStatus(
        session({ pull_request: ready, pull_request_lookup: { ...OK_LOOKUP, stale: true } })
      )
    ).toBeNull();
    expect(
      sessionBoardCardStatus(
        session({
          pull_request: ready,
          pull_request_lookup: { ...OK_LOOKUP, state: "error", cause: "rate_limited" },
        })
      )
    ).toBeNull();
  });

  it("Should give no pill when no facts are known", () => {
    expect(sessionBoardCardStatus(session({ pull_request: null }))).toBeNull();
    expect(sessionBoardCardStatus(session({ pull_request: pr() }))).toBeNull();
  });
});

describe("sessionBoardCardMeta (UT-069)", () => {
  it("Should show the workspace name under All projects when there is no PR", () => {
    const bare = session({ branch: undefined, pull_request: null, pull_request_lookup: null });
    expect(sessionBoardCardMeta(bare, "polygo")).toEqual({
      shape: null,
      branch: null,
      number: null,
      workspaceName: "polygo",
    });
    expect(sessionBoardCardMeta(bare, null)).toMatchObject({ shape: null, workspaceName: null });
  });

  it("Should name the PR shape, branch and number, or the bare branch", () => {
    expect(sessionBoardCardMeta(session({ pull_request: pr({ draft: true }) }), "polygo")).toEqual({
      shape: "draft",
      branch: "feat/kanban",
      number: 931,
      workspaceName: null,
    });
    expect(sessionBoardCardMeta(session({ pull_request: null }), null)).toMatchObject({
      shape: "branch",
      branch: "feat/kanban",
      number: null,
    });
  });
});

describe("sessionBoardCardRollup (UT-070)", () => {
  it("Should count children per column and flag one that needs you", () => {
    const parent = session({
      children: { total: 3, needs_you: 1, working: 2, in_review: 0, done: 0 },
    });
    expect(sessionBoardCardRollup(parent)).toEqual({
      total: 3,
      needsYou: true,
      breakdown: [
        { label: "Needs you", count: 1 },
        { label: "Working", count: 2 },
        { label: "In review", count: 0 },
        { label: "Done", count: 0 },
      ],
    });
    expect(
      sessionBoardCardRollup(
        session({ children: { total: 2, needs_you: 0, working: 2, in_review: 0, done: 0 } })
      )?.needsYou
    ).toBe(false);
    expect(sessionBoardCardRollup(session({ children: null }))).toBeNull();
  });
});

describe("sessionBoardCardWaiting", () => {
  it("Should name why a parked card waits", () => {
    expect(sessionBoardCardWaiting(session({ badge: "waiting", waiting_on: "sessions" }))).toEqual({
      glyph: "delegated",
      word: "Waiting",
    });
    expect(sessionBoardCardWaiting(session({ badge: "waiting", waiting_on: "monitor" }))).toEqual({
      glyph: "monitoring",
      word: "Monitoring",
    });
    expect(sessionBoardCardWaiting(session({ badge: "running" }))).toBeNull();
  });
});

describe("sessionBoardCardLineage", () => {
  const lineage = primarySessionFixture.lineage!;

  it("Should name a child's parent and a fork's or continuation's source", () => {
    expect(sessionBoardCardLineage(session({ parent_title: " Coordinator " }))).toEqual({
      kind: "child",
      verb: "Child of",
      title: "Coordinator",
    });
    expect(
      sessionBoardCardLineage(
        session({ parent_title: "Coordinator", lineage: { ...lineage, kind: "fork" } })
      )
    ).toEqual({ kind: "fork", verb: "Forked from", title: "Coordinator" });
    expect(
      sessionBoardCardLineage(
        session({ parent_title: "Coordinator", lineage: { ...lineage, kind: "continue" } })
      )
    ).toEqual({ kind: "continue", verb: "Continued from", title: "Coordinator" });
    expect(sessionBoardCardLineage(session({ parent_title: "  " }))).toBeNull();
  });

  it("Should still name a fork's or continuation's source when its title is unreadable", () => {
    const source = { ...lineage, parent_session_id: "sess-source" };
    expect(
      sessionBoardCardLineage(session({ parent_title: "", lineage: { ...source, kind: "fork" } }))
    ).toEqual({ kind: "fork", verb: "Forked from", title: "sess-source" });
    expect(
      sessionBoardCardLineage(
        session({
          parent_title: "",
          lineage: { ...source, kind: "continue", origin_agent_name: "codex" },
        })
      )
    ).toEqual({ kind: "continue", verb: "Continued from", title: "codex" });
  });
});
