// Suite: onboarding footer summary
// Invariant: the footer states what Continue will save, and an unmet requirement replaces it.
// Boundary IN: the pure step + view-model -> summary projection.
// Boundary OUT: wizard orchestration and the rendered footer.
import { describe, expect, it } from "vitest";

import { onboardingSummary, type OnboardingSummaryInput } from "../onboarding-summary";

function input(overrides: Partial<OnboardingSummaryInput> = {}): OnboardingSummaryInput {
  return {
    step: 1,
    error: null,
    runtime: {
      providerName: "Claude Code",
      modelName: "Claude Opus 4.8",
      reasoningLabel: "High",
      authMode: "native_cli",
      envVar: "",
    },
    workspaces: [{ name: "compozy" }],
    ...overrides,
  };
}

describe("onboardingSummary", () => {
  it("Should name the provider, model, effort and sign-in on the runtime step", () => {
    expect(onboardingSummary(input())).toEqual({
      label: "Saves as your default",
      value: "Claude Code · Claude Opus 4.8 · High · CLI sign-in",
      tone: "neutral",
    });
  });

  it("Should name the bound environment variable when the key is bound", () => {
    const summary = onboardingSummary(
      input({
        runtime: {
          providerName: "OpenRouter",
          modelName: "Grok 4",
          reasoningLabel: null,
          authMode: "bound_secret",
          envVar: " OPENROUTER_API_KEY ",
        },
      })
    );

    expect(summary.value).toBe("OpenRouter · Grok 4 · OPENROUTER_API_KEY");
  });

  it("Should fall back to a generic bound-key phrase before an env var is typed", () => {
    const summary = onboardingSummary(
      input({
        runtime: {
          providerName: "OpenRouter",
          modelName: "Grok 4",
          reasoningLabel: null,
          authMode: "bound_secret",
          envVar: "",
        },
      })
    );

    expect(summary.value).toBe("OpenRouter · Grok 4 · bound key");
  });

  it("Should say nothing is selected rather than render an empty runtime line", () => {
    const summary = onboardingSummary(
      input({
        runtime: {
          providerName: "",
          modelName: "",
          reasoningLabel: null,
          authMode: "native_cli",
          envVar: "",
        },
      })
    );

    expect(summary).toEqual({
      label: "Saves as your default",
      value: "Nothing selected yet — pick a provider and model.",
      tone: "neutral",
    });
  });

  it("Should count and name the workspaces on the workspace step", () => {
    const summary = onboardingSummary(
      input({ step: 2, workspaces: [{ name: "compozy" }, { name: "infra" }] })
    );

    expect(summary).toEqual({
      label: "Workspaces",
      value: "2 folders · compozy, infra",
      tone: "neutral",
    });
  });

  it("Should tell the operator what is missing when no workspace is selected", () => {
    const summary = onboardingSummary(input({ step: 2, workspaces: [] }));

    expect(summary).toEqual({
      label: "Workspaces",
      value: "None yet — add at least one folder to finish.",
      tone: "neutral",
    });
  });

  it("Should replace the summary with the error on any step", () => {
    const message = "Enter the environment variable the provider expects.";

    for (const step of [1, 2]) {
      expect(onboardingSummary(input({ step, error: message }))).toEqual({
        label: "Needs an answer",
        value: message,
        tone: "danger",
      });
    }
  });

  it("Should ignore a blank error string", () => {
    expect(onboardingSummary(input({ error: "   " })).tone).toBe("neutral");
  });
});
