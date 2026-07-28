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
  it("Should report context, price, tools, reasoning levels and harness in reading order", () => {
    const facts = onboardingModelFacts(
      model({
        context_window: 1_000_000,
        cost_input: 5,
        cost_output: 25,
        supports_tools: true,
        efforts: ["low", "medium", "high"],
      }),
      "acp"
    );

    expect(facts).toEqual([
      { id: "context", value: "1M", label: "context" },
      { id: "price", value: "$5/25", label: "per Mtok" },
      { id: "tools", value: null, label: "tool calls" },
      { id: "reasoning", value: "3", label: "reasoning levels" },
      { id: "harness", value: "CLI", label: "harness" },
    ]);
  });

  it("Should omit every fact the runtime does not report", () => {
    const facts = onboardingModelFacts(
      model({ context_window: null, cost_input: null, cost_output: null, supports_tools: false }),
      null
    );

    expect(facts).toEqual([]);
  });

  it("Should omit price when only one side of the rate is known", () => {
    const facts = onboardingModelFacts(model({ cost_input: 3, cost_output: null }), null);

    expect(facts.map(fact => fact.id)).not.toContain("price");
  });

  it("Should never round a charged sub-cent rate down to zero", () => {
    const facts = onboardingModelFacts(model({ cost_input: 0.002, cost_output: 0 }), null);

    expect(facts).toContainEqual({ id: "price", value: "$<0.01/0", label: "per Mtok" });
  });

  it("Should state reasoning is on when the model exposes no selectable levels", () => {
    const facts = onboardingModelFacts(
      model({ supports_reasoning: true, efforts: ["none"] }),
      "pi_acp"
    );

    expect(facts).toEqual([
      { id: "reasoning", value: null, label: "reasoning on" },
      { id: "harness", value: "API key", label: "harness" },
    ]);
  });

  it("Should shorten large context windows and singularise a lone reasoning level", () => {
    const facts = onboardingModelFacts(
      model({ context_window: 200_000, efforts: ["high"] }),
      "   "
    );

    expect(facts).toEqual([
      { id: "context", value: "200k", label: "context" },
      { id: "reasoning", value: "1", label: "reasoning level" },
    ]);
  });

  it("Should normalise an unrecognised harness instead of inventing a label for it", () => {
    const facts = onboardingModelFacts(undefined, "Custom_Harness");

    expect(facts).toEqual([{ id: "harness", value: "custom_harness", label: "harness" }]);
  });
});
