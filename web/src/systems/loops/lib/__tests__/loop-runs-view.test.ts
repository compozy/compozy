import { describe, expect, it } from "vitest";

import { loopRunFixtures } from "../../mocks/fixtures";
import type { LoopRun } from "../../types";
import {
  buildOutcomeSegments,
  buildRunKpis,
  formatRunInputs,
  formatTokenCount,
  loopBudgetBar,
  partitionRuns,
  runGenerationLabel,
} from "../loop-runs-view";

// A fixed "now" on the fixtures' calendar day so "Done today" is deterministic.
const NOW = new Date("2026-07-05T20:00:00Z");

function run(overrides: Partial<LoopRun> & Pick<LoopRun, "id" | "status">): LoopRun {
  return {
    workspace_id: "ws",
    loop_name: "software-delivery",
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
    ...overrides,
  };
}

describe("loop-runs-view", () => {
  it("Should derive the four workspace KPIs from the run set", () => {
    const kpis = buildRunKpis(loopRunFixtures, NOW);
    // 5 live: running, watching, needs-approval, paused, queued.
    expect(kpis.activeNow.count).toBe(5);
    // 1 needs-approval.
    expect(kpis.awaitingYou.count).toBe(1);
    // 2 done rows, both with last_progress_at on 2026-07-05.
    expect(kpis.doneToday.count).toBe(2);
    // failed(1) + exhausted(2) + stalled(2) + blocked(1) = 6.
    expect(kpis.needsALook.count).toBe(6);
  });

  it("Should count only done runs whose terminal timestamp is the current local day", () => {
    const runs = [
      run({ id: "a", status: "done", last_progress_at: NOW.toISOString() }),
      run({ id: "b", status: "done", last_progress_at: "2026-07-01T09:00:00Z" }),
    ];
    expect(buildRunKpis(runs, NOW).doneToday.count).toBe(1);
  });

  it("Should build a data-driven outcome filter in canonical order with counts", () => {
    const segments = buildOutcomeSegments(loopRunFixtures);
    expect(segments[0]).toEqual({ value: "all", label: "All", count: loopRunFixtures.length });
    const values = segments.map(segment => segment.value);
    // Live states precede terminal states; only present statuses appear.
    expect(values).toContain("running");
    expect(values.indexOf("running")).toBeLessThan(values.indexOf("done"));
    const queued = segments.find(segment => segment.value === "queued");
    expect(queued?.count).toBe(1);
  });

  it("Should split live runs into Active and terminal runs into Past, honoring the outcome filter", () => {
    const { active, past } = partitionRuns(loopRunFixtures);
    expect(active).toHaveLength(5);
    expect(past).toHaveLength(9);
    const onlyDone = partitionRuns(loopRunFixtures, "done");
    expect(onlyDone.active).toHaveLength(0);
    expect(onlyDone.past.every(runItem => runItem.status === "done")).toBe(true);
  });

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
