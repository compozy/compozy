import { describe, expect, it } from "vitest";

import type { SubagentPayload, SubagentWirePayload } from "../../adapters/subagent-api";
import {
  applySubagentsSnapshot,
  applySubagentUpdated,
  EMPTY_SUBAGENT_ROSTER,
  markSubagentRosterStale,
  settledSubagentIds,
} from "../subagent-roster";

// Suite: parent subagent roster reducer.
// Invariant: the snapshot replaces every row; an update upserts by id and never moves a row
// back in time; rows held across a reconnect are stale until a frame confirms them; each row's
// isolation and worktree facts map truthfully (absent ≠ zero, unknown ≠ none, shared = no facts).
// Owning layer: session roster (pure). Canonical suite: this file.
const T0 = Date.parse("2026-10-08T21:00:00.000Z");
const at = (seconds: number) => new Date(T0 + seconds * 1_000).toISOString();

function row(id: string, overrides: Partial<SubagentWirePayload> = {}): SubagentWirePayload {
  return {
    subagent_id: id,
    workspace_id: "ws-01",
    parent_session_id: "sess-parent",
    parent_turn_id: "turn-1",
    child_session_id: `sess-${id}`,
    origin: "delegated",
    provider_tool_call_id: null,
    title: `Task ${id}`,
    role: "general",
    status: "running",
    work_state: "working",
    runtime: { agent: "codex", provider: "codex", model: "", reasoning_effort: "", speed: "" },
    depth: 1,
    progress: "",
    result: null,
    result_preview: "",
    result_truncated: false,
    error: null,
    wait_timed_out: false,
    delivery: "none",
    created_at: at(0),
    started_at: at(0),
    settled_at: null,
    updated_at: at(1),
    ...overrides,
  };
}

const snapshot = (...rows: SubagentPayload[]) =>
  applySubagentsSnapshot({ session_id: "sess-parent", subagents: rows });
const update = (subagent: SubagentPayload) => ({ session_id: "sess-parent", subagent });

describe("subagent roster (UT-W01)", () => {
  it("Should preserve creation time when a queued row receives progress before starting", () => {
    const queued = row("queued", { status: "queued", started_at: null, created_at: at(-10) });
    const roster = applySubagentUpdated(
      snapshot(queued),
      update({ ...queued, progress: "Queued", updated_at: at(20) })
    );
    expect(roster.rows[0]?.created_at).toBe(at(-10));
  });
  it("Should upsert updates by id after the snapshot and prepend rows first seen in an update", () => {
    let roster = snapshot(row("a"), row("b"));
    roster = applySubagentUpdated(
      roster,
      update(row("b", { progress: "Reading", updated_at: at(5) }))
    );
    roster = applySubagentUpdated(roster, update(row("c", { updated_at: at(6) })));
    expect(roster.rows.map(view => view.id)).toEqual(["c", "a", "b"]);
    expect(roster.rows.find(view => view.id === "b")?.progress).toBe("Reading");
    expect(settledSubagentIds(roster)).toEqual([]);
  });

  it("Should ignore an update older than the row it would replace", () => {
    const roster = snapshot(
      row("a", { status: "completed", settled_at: at(9), updated_at: at(9) })
    );
    const stale = applySubagentUpdated(roster, update(row("a", { updated_at: at(4) })));
    expect(stale).toBe(roster);
    expect(stale.rows[0]?.status).toBe("completed");
    expect(settledSubagentIds(stale)).toEqual(["a"]);
  });

  it("Should mark held rows stale on reconnect until an update or the next snapshot confirms them", () => {
    expect(markSubagentRosterStale(EMPTY_SUBAGENT_ROSTER)).toBe(EMPTY_SUBAGENT_ROSTER);
    const held = markSubagentRosterStale(snapshot(row("a"), row("b")));
    expect(held.staleIds).toEqual(new Set(["a", "b"]));
    const confirmed = applySubagentUpdated(held, update(row("a", { updated_at: at(3) })));
    expect(confirmed.staleIds).toEqual(new Set(["b"]));
    expect(snapshot(row("a"), row("b")).staleIds.size).toBe(0);
  });
});

describe("subagent isolation and worktree facts (UT-070, UT-072)", () => {
  const worktree = {
    id: "wt-5d1a7c0e",
    name: "extract-billing-client-3f9a0c12",
    branch: "run/extract-billing-client-3f9a0c12",
    base_ref: "origin/main",
    base_sha: "4be1c9d2a7f0e6b13c5d8e9f0a1b2c3d4e5f6a7b",
    path: "/w/extract-billing-client-3f9a0c12",
  };
  const settled = {
    ...worktree,
    head_sha: "9c41e07",
    commits_ahead: 0,
    dirty_files: 0,
    observed_at: at(60),
  };
  const view = (overrides: Partial<SubagentWirePayload>) => snapshot(row("a", overrides)).rows[0]!;

  it("Should map an isolated payload's worktree and keep unobserved facts absent, not zero", () => {
    const running = view({ isolation: "worktree", worktree });
    expect(running.isolation).toBe("worktree");
    expect(running.worktree).toMatchObject({
      name: worktree.name,
      branch: worktree.branch,
      base_ref: "origin/main",
      commits_ahead: null,
      dirty_files: null,
      observed_at: null,
      pull_request_status: null,
      pull_request: null,
    });
    const done = view({
      isolation: "worktree",
      worktree: {
        ...settled,
        pull_request_status: "open",
        pull_request: {
          url: "https://github.com/compozy/compozy/pull/731",
          number: 731,
          state: "open",
        },
      },
    });
    expect(done.worktree).toMatchObject({
      commits_ahead: 0,
      dirty_files: 0,
      pull_request_status: "open",
      pull_request: { number: 731, state: "open" },
    });
  });

  it("Should keep unknown and none distinct and read an unreadable PR as unknown, never none", () => {
    const status = (worktreePayload: SubagentWirePayload["worktree"]) =>
      view({ isolation: "worktree", worktree: worktreePayload }).worktree;
    expect(status({ ...settled, pull_request_status: "unknown" })).toMatchObject({
      pull_request_status: "unknown",
      pull_request: null,
    });
    expect(status({ ...settled, pull_request_status: "none" })).toMatchObject({
      pull_request_status: "none",
      pull_request: null,
    });
    const unreadable = [
      { ...settled, pull_request_status: "open" },
      {
        ...settled,
        pull_request_status: "merged",
        pull_request: { url: "javascript:alert(1)", number: 7, state: "merged" },
      },
      { ...settled, pull_request_status: "superseded" },
    ];
    for (const payload of unreadable) {
      expect(status(payload)).toMatchObject({ pull_request_status: "unknown", pull_request: null });
    }
  });

  it("Should give shared subagents no facts, including rows from daemons without isolation", () => {
    expect(view({})).toMatchObject({ isolation: "shared", worktree: null });
    expect(view({ isolation: "shared", worktree })).toMatchObject({
      isolation: "shared",
      worktree: null,
    });
  });
});
