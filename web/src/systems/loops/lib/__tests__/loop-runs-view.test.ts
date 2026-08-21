import { describe, expect, it } from "vitest";

import { loopRunFixtures } from "../../mocks/fixtures";
import type { LoopRun } from "../../types";
import {
  buildRunsRoster,
  formatRunInputs,
  formatTokenCount,
  loopBudgetBar,
  loopRunOriginLine,
  runGenerationLabel,
} from "../loop-runs-view";

function run(overrides: Partial<LoopRun> & Pick<LoopRun, "id" | "status">): LoopRun {
  return {
    workspace_id: "ws",
    loop_name: "implement-tasks",
    completion_state: "complete",
    generation: 1,
    iteration_cap: 50,
    tokens_used: 0,
    pause_requested: false,
    budget_tokens: 0,
    budget_wall_sec: 0,
    budget_on_exceeded: "halt",
    reattempt_strategy: "failed_only",
    resolved_network_participation: null,
    created_at: "2026-07-05T12:00:00Z",
    started_at: "2026-07-05T12:00:00Z",
    last_progress_at: "2026-07-05T12:00:00Z",
    definition_version: 1,
    progress: { round: 1, steps_done: 0, steps_total: 0 },
    ...overrides,
    forks: overrides.forks ?? [],
  };
}

// UT-045. The ordering itself is the daemon's — SQL ranks needs-you above live
// above terminal before the page is cut. What these cases pin is that the page
// groups that ranking without re-deciding it, and that an empty roster explains
// how to start rather than showing a blank table.
describe("buildRunsRoster", () => {
  it("Should group needs-you first, then active, then recent", () => {
    const model = buildRunsRoster([
      run({
        id: "needs",
        status: "running",
        attention: { kind: "approval", count: 1, since: "2026-07-05T11:57:00Z" },
        active_gate_id: "aplicar-correcoes",
        progress: { round: 1, steps_done: 4, steps_total: 6 },
      }),
      run({ id: "live", status: "running", progress: { round: 1, steps_done: 2, steps_total: 9 } }),
      run({ id: "done", status: "done", progress: { round: 2, steps_done: 6, steps_total: 6 } }),
    ]);

    expect(model.groups.map(group => group.id)).toEqual(["needs-you", "active", "recent"]);
    expect(model.groups.map(group => group.label)).toEqual(["Needs you", "Active", "Recent"]);
    expect(model.needsYouCount).toBe(1);
    expect(model.total).toBe(3);
  });

  it("Should preserve the server's order inside a group rather than sorting it", () => {
    const model = buildRunsRoster([
      run({ id: "second", status: "running" }),
      run({ id: "first", status: "watching" }),
      run({ id: "third", status: "queued" }),
    ]);

    // Alphabetical or status-based reordering here would quietly contradict the
    // ranking the daemon applied before pagination.
    expect(model.groups[0].rows.map(row => row.run.id)).toEqual(["second", "first", "third"]);
  });

  it("Should lead a needs-you row with what is being asked, not with the mechanism", () => {
    const model = buildRunsRoster([
      run({
        id: "needs",
        status: "running",
        attention: { kind: "approval", count: 1, since: "2026-07-05T11:57:00Z" },
        active_gate_id: "aplicar-correcoes",
        progress: { round: 1, steps_done: 4, steps_total: 6 },
      }),
    ]);

    const row = model.groups[0].rows[0];
    expect(row.needsYou).toBe(true);
    expect(row.statusLabel).toBe("Needs you");
    // Warning on the page; danger stays with failure and the attention bell.
    expect(row.statusTone).toBe("warning");
    expect(row.summaryLine).toBe("an approval is waiting on “aplicar-correcoes”");
    expect(row.progressLabel).toBe("step 4 of 6");
  });

  it("Should never sort a needs-you run below a terminal one", () => {
    const model = buildRunsRoster([
      run({ id: "done", status: "done" }),
      run({
        id: "needs",
        status: "running",
        attention: { kind: "request", count: 2, since: "2026-07-05T11:00:00Z" },
      }),
    ]);

    // Even when the read hands them over terminal-first, grouping puts the run
    // that needs a person at the top of the page.
    expect(model.groups[0].id).toBe("needs-you");
    expect(model.groups[0].rows[0].run.id).toBe("needs");
  });

  it("Should count rounds on a terminal run and steps while it is live", () => {
    const model = buildRunsRoster([
      run({ id: "live", status: "running", progress: { round: 2, steps_done: 1, steps_total: 4 } }),
      run({ id: "done", status: "done", progress: { round: 2, steps_done: 6, steps_total: 6 } }),
      run({ id: "cold", status: "queued", progress: { round: 1, steps_done: 0, steps_total: 0 } }),
    ]);

    const rows = model.groups.flatMap(group => group.rows);
    expect(rows.find(row => row.run.id === "live")?.progressLabel).toBe("step 1 of 4 · round 2");
    expect(rows.find(row => row.run.id === "done")?.progressLabel).toBe("2 rounds");
    expect(rows.find(row => row.run.id === "cold")?.progressLabel).toBe("not started");
  });

  it("Should explain how to start when the workspace has no runs at all", () => {
    const model = buildRunsRoster([]);

    expect(model.groups).toEqual([]);
    expect(model.emptyState?.title).toBe("No runs yet");
    expect(model.emptyState?.body).toContain("Start a loop from the catalog");
    expect(model.emptyState?.actionLabel).toBe("Browse loops");
  });

  it("Should distinguish a filtered-empty roster from a genuinely empty one", () => {
    const model = buildRunsRoster([run({ id: "a", status: "running" })], "failed");
    // "No runs yet" would be a lie here: there are runs, just none matching.
    expect(model.emptyState?.title).toBe("No runs match this filter");
    expect(model.emptyState?.actionLabel).toBe("Clear filter");
  });

  it("Should apply the outcome filter across the whole roster", () => {
    const model = buildRunsRoster(loopRunFixtures, "done");
    const rows = model.groups.flatMap(group => group.rows);
    expect(rows.length).toBeGreaterThan(0);
    expect(rows.every(row => row.run.status === "done")).toBe(true);
  });
});

describe("loop-runs-view", () => {
  it("Should render generation vs cap and compact token counts", () => {
    expect(runGenerationLabel({ generation: 2, iteration_cap: 50 })).toBe("2 / 50");
    expect(runGenerationLabel({ generation: 5, iteration_cap: 0 })).toBe("5 / ∞");
    expect(formatTokenCount(412_000)).toBe("412K");
    expect(formatTokenCount(2_400_000)).toBe("2.4M");
    expect(formatTokenCount(0)).toBe("0");
  });

  it("Should preview resolved inputs truthfully", () => {
    expect(formatRunInputs({ slug: "x", branch: "main", extra: 1 })).toBe("slug: x · branch: main");
    expect(formatRunInputs(undefined)).toBe("");
  });

  it("Should keep origin kind and reference from the same recorded pair", () => {
    expect(
      loopRunOriginLine({
        started_origin_kind: "schedule",
        started_origin_ref: "nightly",
        started_by_kind: "user",
        started_by_ref: "pedro",
      })
    ).toBe("schedule · nightly");
    expect(
      loopRunOriginLine({
        started_origin_kind: "schedule",
        started_origin_ref: "",
        started_by_kind: "user",
        started_by_ref: "pedro",
      })
    ).toBe("user · pedro");
  });

  it("Should model the budget mini-bar: uncapped shows no percent, capped warns and dangers near the ceiling", () => {
    expect(loopBudgetBar({ tokens_used: 12_000, budget_tokens: 0 })).toEqual({
      tokensLabel: "12K tok",
      hasCap: false,
      percent: null,
      tone: "neutral",
    });
    expect(loopBudgetBar({ tokens_used: 520_000, budget_tokens: 2_000_000 })).toMatchObject({
      hasCap: true,
      percent: 26,
      tone: "neutral",
    });
    expect(loopBudgetBar({ tokens_used: 1_900_000, budget_tokens: 2_000_000 }).tone).toBe("warn");
    expect(loopBudgetBar({ tokens_used: 2_000_000, budget_tokens: 2_000_000 })).toMatchObject({
      percent: 100,
      tone: "danger",
    });
  });
});
