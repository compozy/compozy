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

  it("Should pair each per-loop default with its hard daemon ceiling", () => {
    const rows = buildLoopLimits(baseEffectiveConfig);
    const byLabel = new Map(rows.map(row => [row.label, row]));
    expect(byLabel.get("Iteration cap")).toMatchObject({ value: "50", ceiling: "/ 100" });
    expect(byLabel.get("Token budget")).toMatchObject({ value: "off", ceiling: "/ 20M" });
    expect(byLabel.get("Wall clock")).toMatchObject({ value: "off", ceiling: "/ 7d" });
    expect(byLabel.get("On exceeded")).toMatchObject({ value: "halt", ceiling: "→ exhausted" });
    expect(byLabel.get("Cost (USD)")).toMatchObject({ value: "—", ceiling: "display-only" });
    expect(byLabel.get("Fan-out breadth")).toMatchObject({ value: "4", ceiling: "/ 64" });
  });

  it("Should render the unbounded glyph for watch loops and escalate targets", () => {
    const rows = buildLoopLimits({
      ...baseEffectiveConfig,
      iteration_cap: 0,
      budget_on_exceeded: "escalate" as const,
    });
    const byLabel = new Map(rows.map(row => [row.label, row]));
    expect(byLabel.get("Iteration cap")?.value).toBe("∞");
    expect(byLabel.get("On exceeded")).toMatchObject({
      value: "escalate",
      ceiling: "→ needs-approval",
    });
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

    expect(byLabel.get("Iteration cap")?.value).toBe("3");
    expect(byLabel.get("No-progress window")?.value).toBe("2");
    expect(byLabel.get("Fan-out breadth")?.value).toBe("4");
    expect(byLabel.get("Gate max revisions")?.value).toBe("2");
    expect(byLabel.get("On exceeded")?.value).toBe("escalate");
  });
});
