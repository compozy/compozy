import { describe, expect, it } from "vitest";

import { loopEffectiveConfigFixture } from "../../mocks/fixtures";
import type { LoopDryRunPreview, LoopInputSchema } from "../../types";
import { loopRunFormLogic } from "../use-loop-run-form-state";

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
  nodes: [],
  resolved_inputs: {},
  resolved_network_participation: {
    mode: "local",
    source: "built_in_local",
    version: "network-participation/v1",
  },
};

describe("loopRunFormLogic", () => {
  // Invariant: an explicit participation edit and any submitted dry run belong to
  // the active run-form draft; a draft edit rejects a late dry-run plan. Owning
  // layer: unit transition logic. Canonical suite: this file, because no existing
  // run-form state-transition suite owns the overlay orchestration.
  it("marks form intent and rejects a late plan after participation changes", () => {
    const store = loopRunFormLogic.createStore({
      effectiveConfig: loopEffectiveConfigFixture,
      networkParticipation: { channelId: "", channelStrategy: "", mode: "local" },
      schema,
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
});
