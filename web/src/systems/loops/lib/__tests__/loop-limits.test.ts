import { describe, expect, it } from "vitest";

import { loopEffectiveConfigFixture } from "../../mocks/fixtures";
import { buildLoopLimits, formatTokenBudget, formatWallClock } from "../loop-limits";

const baseEffectiveConfig = {
  ...loopEffectiveConfigFixture,
  iteration_cap: 50,
  budget_tokens: 0,
  budget_wall_sec: 0,
  budget_on_exceeded: "halt" as const,
  no_progress_window: 3,
};

describe("loop-limits", () => {
  it("Should format token budgets compactly and off when unset", () => {
    expect(formatTokenBudget(0)).toBe("off");
    expect(formatTokenBudget(500_000)).toBe("500K");
    expect(formatTokenBudget(20_000_000)).toBe("20M");
    expect(formatTokenBudget(2_400_000)).toBe("2.4M");
  });

  it("Should format wall-clock budgets, rendering off when unset", () => {
    expect(formatWallClock(0)).toBe("off");
    expect(formatWallClock(604_800)).toBe("7d");
    expect(formatWallClock(3_600)).toBe("1h");
    expect(formatWallClock(90)).toBe("2m");
  });

  it("Should pair each per-loop default with its hard ceiling in plain words", () => {
    const rows = buildLoopLimits(baseEffectiveConfig);
    const byLabel = new Map(rows.map(row => [row.label, row]));
    expect(byLabel.get("Max rounds")).toMatchObject({ value: "50", ceiling: "Up to 100" });
    expect(byLabel.get("Token budget")).toMatchObject({ value: "off", ceiling: "Up to 20M" });
    expect(byLabel.get("Time limit")).toMatchObject({ value: "off", ceiling: "Up to 7d" });
    expect(byLabel.get("When a budget runs out")).toMatchObject({ value: "Stop the run" });
    expect(byLabel.has("Cost (USD)")).toBe(false);
    expect(byLabel.get("Parallel workers")).toMatchObject({
      value: "4",
      ceiling: "No fixed limit",
    });
  });

  it("Should render the unbounded glyph for watch loops and the escalate policy", () => {
    const rows = buildLoopLimits({
      ...baseEffectiveConfig,
      iteration_cap: 0,
      budget_on_exceeded: "escalate" as const,
    });
    const byLabel = new Map(rows.map(row => [row.label, row]));
    expect(byLabel.get("Max rounds")?.value).toBe("∞");
    expect(byLabel.get("When a budget runs out")?.value).toBe("Pause and ask me");
  });

  it("Should render saved per-Loop limits instead of authored defaults", () => {
    const rows = buildLoopLimits({
      ...baseEffectiveConfig,
      iteration_cap: 3,
      budget_on_exceeded: "escalate" as const,
      no_progress_window: 2,
      fan_out_width: 4,
      gate_max_revisions: 2,
    });
    const byLabel = new Map(rows.map(row => [row.label, row]));

    expect(byLabel.get("Max rounds")?.value).toBe("3");
    expect(byLabel.get("Rounds without progress")?.value).toBe("2");
    expect(byLabel.get("Parallel workers")?.value).toBe("4");
    expect(byLabel.get("Max revision requests")?.value).toBe("2");
    expect(byLabel.get("When a budget runs out")?.value).toBe("Pause and ask me");
  });
});
