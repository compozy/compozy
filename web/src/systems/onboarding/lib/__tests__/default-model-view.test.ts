// Suite: Onboarding runtime provider options (UT-028)
// Invariant: the default-model picker offers only enabled providers; a missing `enabled` reads as on.
import { describe, expect, it } from "vitest";

import type { ProviderSummary } from "@/systems/providers";

import { onboardingRuntimeProviders } from "../default-model-view";

function summary(name: string, enabled?: boolean): ProviderSummary {
  return {
    name,
    default: false,
    reviewer_mode: "auto",
    auth_status: {
      mode: "none",
      env_policy: "filtered",
      home_policy: "operator",
      state: "none",
      login: { configured: false, presence: "unknown" },
    },
    ...(enabled === undefined ? {} : { enabled }),
  } as ProviderSummary;
}

describe("onboardingRuntimeProviders", () => {
  it("Should drop providers turned off in Settings (UT-028)", () => {
    const options = onboardingRuntimeProviders([
      summary("claude", true),
      summary("gemini", false),
      summary("legacy"),
    ]);
    expect(options.map(option => option.id)).toEqual(["claude", "legacy"]);
  });
});
