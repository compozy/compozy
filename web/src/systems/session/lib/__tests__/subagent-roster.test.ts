import { describe, expect, it } from "vitest";

import type { SubagentPayload } from "../../adapters/subagent-api";
import {
  applySubagentsSnapshot,
  applySubagentUpdated,
  EMPTY_SUBAGENT_ROSTER,
  markSubagentRosterStale,
  settledSubagentIds,
} from "../subagent-roster";

// Suite: parent subagent roster reducer.
// Invariant: the snapshot replaces every row; an update upserts by id and never moves a row
// back in time; rows held across a reconnect are stale until a frame confirms them.
// Owning layer: session roster (pure). Canonical suite: this file.
const T0 = Date.parse("2026-10-08T21:00:00.000Z");
const at = (seconds: number) => new Date(T0 + seconds * 1_000).toISOString();

function row(id: string, overrides: Partial<SubagentPayload> = {}): SubagentPayload {
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
  it("Should upsert updates by id after the snapshot and prepend rows first seen in an update", () => {
    let roster = snapshot(row("a"), row("b"));
    roster = applySubagentUpdated(
      roster,
      update(row("b", { progress: "Reading", updated_at: at(5) }))
    );
    roster = applySubagentUpdated(roster, update(row("c", { updated_at: at(6) })));
    expect(roster.rows.map(view => view.id)).toEqual(["c", "a", "b"]);
    expect(roster.rows.find(view => view.id === "b")?.progress).toBe("Reading");
    expect(settledSubagentIds(roster).size).toBe(0);
  });

  it("Should ignore an update older than the row it would replace", () => {
    const roster = snapshot(
      row("a", { status: "completed", settled_at: at(9), updated_at: at(9) })
    );
    const stale = applySubagentUpdated(roster, update(row("a", { updated_at: at(4) })));
    expect(stale).toBe(roster);
    expect(stale.rows[0]?.status).toBe("completed");
    expect(settledSubagentIds(stale)).toEqual(new Set(["a"]));
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
