// Suite: onboarding model facts
// Invariant: the facts strip states only what the runtime actually reports about a model.
// Boundary IN: the pure model -> fact list projection.
// Boundary OUT: the model catalog transport and the rendered strip.
import { describe, expect, it } from "vitest";

import type { RuntimeModelOption } from "@/systems/runtime";

import { onboardingModelFacts } from "../model-facts";

function model(overrides: Partial<RuntimeModelOption> = {}): RuntimeModelOption {
  return {
    id: "claude-opus-4-8",
    provider: "claude",
    name: "Claude Opus 4.8",
    availability: "live",
    efforts: [],
    ...overrides,
  };
}

describe("onboardingModelFacts", () => {
  it("Should state only the price in plain words", () => {
    const facts = onboardingModelFacts(
      model({
        context_window: 1_000_000,
        cost_input: 5,
        cost_output: 25,
        supports_tools: true,
        efforts: ["low", "medium", "high"],
      })
    );

    expect(facts).toEqual([{ id: "price", value: "$5 in / $25 out", label: "per million tokens" }]);
  });

  it("Should omit the price when the runtime does not report it", () => {
    expect(onboardingModelFacts(model({ cost_input: null, cost_output: null }))).toEqual([]);
    expect(onboardingModelFacts(undefined)).toEqual([]);
  });

  it("Should omit price when only one side of the rate is known", () => {
    expect(onboardingModelFacts(model({ cost_input: 3, cost_output: null }))).toEqual([]);
  });

  it("Should never round a charged sub-cent rate down to zero", () => {
    const facts = onboardingModelFacts(model({ cost_input: 0.002, cost_output: 0 }));

    expect(facts).toContainEqual({
      id: "price",
      value: "$<0.01 in / $0 out",
      label: "per million tokens",
    });
  });
});
