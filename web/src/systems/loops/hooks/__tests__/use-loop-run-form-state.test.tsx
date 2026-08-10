// Suite: loop run-form state scope binding
// Invariant: a run-form draft belongs to exactly one workspace-and-loop scope; when
// that identity changes, the next scope receives its own baseline instead of the old draft.
// Owning layer: hook integration. Canonical suite: this file (no existing run-form-state suite).
import { act, renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { loopEffectiveConfigFixture } from "../../mocks/fixtures";
import type { LoopDryRunPreview, LoopInputSchema } from "../../types";
import { loopRunFormLogic, useLoopRunFormState } from "../use-loop-run-form-state";

interface FormScope {
  workspaceId: string;
  loopName: string;
}

function formInput(scope: FormScope, channelId: string) {
  return {
    effectiveConfig: loopEffectiveConfigFixture,
    networkParticipation: { mode: "live" as const, channelId, channelStrategy: "named" as const },
    schema: undefined,
    scope,
  };
}

const schema: LoopInputSchema = {
  goal: { required: true, type: "string" },
};

const plan: LoopDryRunPreview = {
  contract: {
    budget: { on_exceeded: "halt", tokens: 100, wall_clock_sec: 60 },
    definition_of_done: "Done.",
    goal: "Ship it.",
    iteration_cap: 1,
    no_progress: { window: 1 },
  },
  effective_config: loopEffectiveConfigFixture,
  generation: 1,
  input_origins: {},
  loop_name: "delivery",
  materialized_contract: {
    budget: { on_exceeded: "halt", tokens: 100, wall_clock_sec: 60 },
    definition_of_done: "Done.",
    goal: "Ship it.",
    iteration_cap: 1,
    no_progress: { window: 1 },
  },
  nodes: [],
  resolved_inputs: {},
  resolved_network_participation: {
    mode: "local",
    source: "built_in_local",
    version: "network-participation/v1",
  },
};

describe("useLoopRunFormState", () => {
  it("Should reject a late plan after the active draft changes", () => {
    const store = loopRunFormLogic.createStore({
      effectiveConfig: loopEffectiveConfigFixture,
      networkParticipation: { channelId: "", channelStrategy: "", mode: "local" },
      schema,
      scope: { loopName: "delivery", workspaceId: "workspace-alpha" },
    });

    store.trigger.dryRunRequested({ execute: () => new Promise<never>(() => undefined) });
    expect(store.getSnapshot().context.submitAttempted).toBe(true);

    store.trigger.networkParticipationChanged({
      draft: { channelId: "release", channelStrategy: "named", mode: "live" },
    });
    expect(store.getSnapshot().context.networkParticipationOverridden).toBe(true);

    store.trigger.dryRunSucceeded({ attempt: 1, generation: 1, plan });
    expect(store.getSnapshot().context.plan).toBeNull();

    store.trigger.dryRunRequested({ execute: () => new Promise<never>(() => undefined) });
    store.trigger.dryRunSucceeded({ attempt: 2, generation: 3, plan });
    expect(store.getSnapshot().context.plan).toEqual(plan);
  });

  it("Should replace the draft when the workspace-and-loop scope changes", () => {
    const { result, rerender } = renderHook(
      ({ scope, channelId }: { scope: FormScope; channelId: string }) =>
        useLoopRunFormState(formInput(scope, channelId)),
      {
        initialProps: {
          scope: { workspaceId: "workspace-alpha", loopName: "review" },
          channelId: "channel-alpha",
        },
      }
    );

    act(() => {
      result.current.setNetworkParticipation({
        mode: "live",
        channelId: "edited-alpha",
        channelStrategy: "named",
      });
    });

    rerender({
      scope: { workspaceId: "workspace-beta", loopName: "review" },
      channelId: "channel-beta",
    });

    expect(result.current.networkParticipation).toEqual({
      mode: "live",
      channelId: "channel-beta",
      channelStrategy: "named",
    });
    expect(result.current.networkParticipationOverridden).toBe(false);
  });
});
