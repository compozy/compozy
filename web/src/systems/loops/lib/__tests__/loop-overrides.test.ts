import { describe, expect, it } from "vitest";

import {
  buildConfigOverrides,
  buildOverrideFields,
  clampOverrideValue,
  hasActiveOverrides,
  initialOverrideDraft,
} from "../loop-overrides";
import { loopEffectiveConfigFixture } from "../../mocks/fixtures";

const effectiveConfig = {
  ...loopEffectiveConfigFixture,
  iteration_cap: 50,
  budget_tokens: 500_000,
  budget_wall_sec: 3_600,
  budget_on_exceeded: "halt" as const,
  no_progress_window: 3,
  fan_out_width: 4,
  gate_max_revisions: 10,
  human_gate_enabled: false,
};
const savedEffectiveConfig = {
  ...effectiveConfig,
  iteration_cap: 3,
  budget_on_exceeded: "escalate" as const,
  no_progress_window: 2,
  gate_max_revisions: 2,
  reattempt_strategy: "full_body" as const,
  human_gate_enabled: true,
};

describe("loop-overrides model", () => {
  it("Should build exactly the 6 clamped fields with no cost cap", () => {
    const fields = buildOverrideFields(effectiveConfig);
    expect(fields.map(field => field.key)).toEqual([
      "iteration_cap",
      "budget_tokens",
      "budget_wall_sec",
      "no_progress_window",
      "fan_out_width",
      "gate_max_revisions",
    ]);
    expect(fields.some(field => field.label.toLowerCase().includes("cost"))).toBe(false);
  });

  it("Should read per-loop defaults from the contract (wall clock in minutes)", () => {
    const fields = buildOverrideFields(effectiveConfig);
    const byKey = Object.fromEntries(fields.map(field => [field.key, field.defaultValue]));
    expect(byKey.iteration_cap).toBe(50);
    expect(byKey.budget_tokens).toBe(500_000);
    expect(byKey.budget_wall_sec).toBe(60); // 3600s / 60
    expect(byKey.no_progress_window).toBe(3);
    expect(byKey.fan_out_width).toBe(4);
  });

  it("Should clamp a value to [0, ceiling]", () => {
    const [iteration] = buildOverrideFields(effectiveConfig);
    expect(clampOverrideValue(iteration, 150)).toBe(100);
    expect(clampOverrideValue(iteration, -5)).toBe(0);
    expect(clampOverrideValue(iteration, 42)).toBe(42);
  });

  it("Should flip the overrides-set badge only when a value diverges from its default", () => {
    const draft = initialOverrideDraft(effectiveConfig);
    expect(hasActiveOverrides(draft, effectiveConfig)).toBe(false);
    expect(hasActiveOverrides({ ...draft, values: { iteration_cap: 50 } }, effectiveConfig)).toBe(
      false
    );
    expect(hasActiveOverrides({ ...draft, values: { iteration_cap: 80 } }, effectiveConfig)).toBe(
      true
    );
    expect(hasActiveOverrides({ ...draft, budgetOnExceeded: "escalate" }, effectiveConfig)).toBe(
      true
    );
  });

  it("Should treat 0 as the default on an off-by-default budget field (no redundant override)", () => {
    // reviews-watch's budgets are off (defaultValue null), so 0 == off == default.
    const watchEffectiveConfig = {
      ...effectiveConfig,
      budget_tokens: 0,
      budget_wall_sec: 0,
      iteration_cap: 0,
    };
    const zeroTokens = { values: { budget_tokens: 0 }, budgetOnExceeded: "halt" as const };
    expect(hasActiveOverrides(zeroTokens, watchEffectiveConfig)).toBe(false);
    expect(buildConfigOverrides(zeroTokens, watchEffectiveConfig)).toBeNull();
    expect(
      hasActiveOverrides(
        { values: { budget_tokens: 100 }, budgetOnExceeded: "halt" },
        watchEffectiveConfig
      )
    ).toBe(true);
  });

  it("Should project only changed fields into config_overrides, converting wall minutes to seconds", () => {
    const draft = initialOverrideDraft(effectiveConfig);
    expect(buildConfigOverrides(draft, effectiveConfig)).toBeNull();
    const overrides = buildConfigOverrides(
      { values: { iteration_cap: 80, budget_wall_sec: 120 }, budgetOnExceeded: "escalate" },
      effectiveConfig
    );
    expect(overrides).toEqual({
      iteration_cap: 80,
      budget_wall_sec: 7_200,
      budget_on_exceeded: "escalate",
    });
  });

  it("Should distinguish saved Loop defaults from explicit per-run overrides", () => {
    const draft = initialOverrideDraft(savedEffectiveConfig);
    expect(hasActiveOverrides(draft, savedEffectiveConfig)).toBe(false);
    expect(
      buildConfigOverrides({ ...draft, values: { iteration_cap: 3 } }, savedEffectiveConfig)
    ).toBeNull();
    expect(
      buildConfigOverrides({ ...draft, values: { iteration_cap: 4 } }, savedEffectiveConfig)
    ).toEqual({ iteration_cap: 4 });
  });
});
