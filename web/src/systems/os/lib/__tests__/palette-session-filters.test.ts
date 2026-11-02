// Suite: palette Sessions view filters
// Invariant: every state chip resolves through the one badge dictionary, Idle
// means the idle badge and never the runtime-health or unknown states that
// share its band, counts describe the whole scope rather than a page, and a
// query narrows by exactly what the row shows.
// Boundary IN: chip predicates, chip counts, and query matching.
// Boundary OUT: ordering, data fetching, and rendered chip behavior.
import { describe, expect, it } from "vitest";

import type { SessionPayload } from "@/systems/session";

import type { OsPaletteSessionResult } from "../../hooks/use-os-palette-entities";
import {
  filterPaletteSessions,
  keepServerSessions,
  matchesPaletteSessionQuery,
  paletteSessionFilter,
  paletteSessionFilterCounts,
} from "../palette-session-filters";

function session(overrides: Partial<SessionPayload> = {}): SessionPayload {
  return {
    board_column: "done",
    pull_request: null,
    pull_request_lookup: null,
    supervision: null,
    scope: "workspace",
    effective_workspace_access: "",
    profile_name: "default",
    profile_id: "00000000000000000000000000",
    id: "session-1",
    name: "Refactor session store",
    agent_name: "claude",
    runtime: {
      status: "ready",
      transition: "initial_bind",
      effective: { provider: "claude" },
      selection_revision: 0,
    },
    workspace_id: "workspace-1",
    workspace_path: "/workspace/compozy",
    state: "active",
    badge: "idle",
    attachable: true,
    available_commands: [],
    created_at: "2026-07-20T12:00:00Z",
    updated_at: "2026-07-20T12:01:00Z",
    ...overrides,
    archived_at: overrides.archived_at ?? null,
    pending_interactions: overrides.pending_interactions ?? [],
  };
}

const CATALOG: SessionPayload[] = [
  session({ id: "s-input", badge: "waiting-for-input", name: "Chargeback flow" }),
  session({ id: "s-auth", badge: "waiting-for-auth", name: "Rotate tokens" }),
  session({ id: "s-failed", badge: "failed", name: "TF upgrade", agent_name: "codex" }),
  session({ id: "s-running", badge: "running", name: "Fix payment retries" }),
  session({ id: "s-done", badge: "done", name: "Release notes draft", agent_name: "gemini" }),
  session({ id: "s-idle", badge: "idle", name: "Regen OpenAPI types" }),
  session({ id: "s-hung", badge: "hung", name: "Stuck indexer" }),
  session({ id: "s-unhealthy", badge: "unhealthy", name: "Flaky bridge" }),
  session({ id: "s-stopped", badge: "stopped", name: "Archived sweep" }),
  session({
    id: "s-unknown",
    badge: "not-a-badge" as SessionPayload["badge"],
    name: "Mystery run",
  }),
];

function idsFor(filterId: Parameters<typeof paletteSessionFilter>[0]): string[] {
  return filterPaletteSessions(CATALOG, filterId, "").map(entry => entry.id);
}

describe("palette session filters (UT-061)", () => {
  it("Should class each chip through the badge dictionary", () => {
    expect(idsFor("needs-you")).toEqual(["s-input", "s-auth", "s-failed"]);
    expect(idsFor("working")).toEqual(["s-running"]);
    expect(idsFor("finished")).toEqual(["s-done"]);
    expect(idsFor("all")).toHaveLength(CATALOG.length);
  });

  it("Should keep Idle to the idle badge alone, leaving the rest reachable under All", () => {
    expect(idsFor("idle")).toEqual(["s-idle"]);
    for (const id of ["s-hung", "s-unhealthy", "s-stopped", "s-unknown"]) {
      expect(idsFor("all")).toContain(id);
    }
  });

  it("Should count every chip over the whole scope, zeros included", () => {
    expect(paletteSessionFilterCounts(CATALOG)).toEqual({
      all: 10,
      "needs-you": 3,
      working: 1,
      finished: 1,
      idle: 1,
    });
    expect(paletteSessionFilterCounts([])).toEqual({
      all: 0,
      "needs-you": 0,
      working: 0,
      finished: 0,
      idle: 0,
    });
  });

  it("Should narrow by title or agent, case-insensitively, and treat blank as no filter", () => {
    const row = session({ name: "Chargeback flow", agent_name: "claude" });
    expect(matchesPaletteSessionQuery(row, "CHARGE")).toBe(true);
    expect(matchesPaletteSessionQuery(row, "clau")).toBe(true);
    expect(matchesPaletteSessionQuery(row, "   ")).toBe(true);
    expect(matchesPaletteSessionQuery(row, "webhook")).toBe(false);
  });

  it("Should apply the chip and the query together", () => {
    expect(filterPaletteSessions(CATALOG, "needs-you", "codex").map(entry => entry.id)).toEqual([
      "s-failed",
    ]);
    expect(filterPaletteSessions(CATALOG, "finished", "codex")).toEqual([]);
  });

  // Invariant: a session the server found by a fact the ranker can't read stays in the results.
  describe("server search results", () => {
    const row = (sessionId: string) =>
      ({ sessionId, title: sessionId, agentName: "claude" }) as OsPaletteSessionResult;

    it("Should keep the rows ranking could not read, after the ones it could", () => {
      const ranked = { rows: [row("title-match")], total: 1 };
      const result = keepServerSessions(ranked, [row("pr-match"), row("title-match")], 6);
      expect(result.rows.map(entry => entry.sessionId)).toEqual(["title-match", "pr-match"]);
      expect(result.total).toBe(2);
    });

    it("Should hold the section's cap and still count every match", () => {
      const rows = ["a", "b", "c"].map(row);
      const result = keepServerSessions({ rows: [], total: 0 }, rows, 2);
      expect(result.rows).toHaveLength(2);
      expect(result.total).toBe(3);
    });

    it("Should keep every server row without ranking signals", () => {
      const result = keepServerSessions(
        { rows: [row("a")], total: 1 },
        [row("a"), row("b")],
        undefined
      );
      expect(result.rows.map(entry => entry.sessionId)).toEqual(["a", "b"]);
      expect(result.total).toBe(2);
    });

    it("Should not count a row twice when the ranking cap cut it", () => {
      const rows = ["a", "b", "c", "d"].map(row);
      const result = keepServerSessions({ rows: rows.slice(0, 2), total: 4 }, rows, 2);
      expect(result.total).toBe(4);
    });
  });
});
