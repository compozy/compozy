// Suite: loop run-form state scope binding
// Invariant: a run-form draft belongs to exactly one workspace-and-loop scope; when
// that identity changes, the next scope receives its own baseline instead of the old draft.
// Owning layer: hook integration. Canonical suite: this file (no existing run-form-state suite).
import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const notifyUser = vi.hoisted(() => vi.fn());
vi.mock("@/lib/user-feedback", () => ({ notifyUser }));

import { LoopInputValidationError } from "../../adapters/loops-api";
import { loopEffectiveConfigFixture } from "../../mocks/fixtures";
import type { LoopDryRunPreview, LoopInputSchema } from "../../types";
import { loopRunFormLogic, useLoopRunFormState } from "../use-loop-run-form-state";

interface FormScope {
  workspaceId: string;
  loopName: string;
}

function formInput(scope: FormScope) {
  return {
    effectiveConfig: loopEffectiveConfigFixture,
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
};

describe("useLoopRunFormState", () => {
  it("Should own a daemon input rejection until that field changes", () => {
    const store = loopRunFormLogic.createStore({
      effectiveConfig: loopEffectiveConfigFixture,
      schema,
      scope: { loopName: "delivery", workspaceId: "workspace-alpha" },
    });
    store.trigger.runRequested({
      announceOwner: false,
      execute: () => new Promise<never>(() => undefined),
      loopName: "delivery",
    });
    store.trigger.runFailed({
      attempt: 1,
      loopName: "delivery",
      error: new LoopInputValidationError({
        field: "goal",
        kind: "string",
        loop: "delivery",
        origin: "run",
        reason: "type_mismatch",
      }),
    });

    expect(store.getSnapshot().context.fieldErrors).toEqual({
      goal: "goal has an invalid value.",
    });

    store.trigger.inputChanged({ name: "goal", value: "Ship it" });
    expect(store.getSnapshot().context.fieldErrors).toEqual({});
  });

  it("Should reject a late plan after the active draft changes", () => {
    const store = loopRunFormLogic.createStore({
      effectiveConfig: loopEffectiveConfigFixture,
      schema,
      scope: { loopName: "delivery", workspaceId: "workspace-alpha" },
    });

    store.trigger.dryRunRequested({ execute: () => new Promise<never>(() => undefined) });
    expect(store.getSnapshot().context.submitAttempted).toBe(true);

    store.trigger.inputChanged({ name: "goal", value: "Updated goal" });
    store.trigger.dryRunSucceeded({ attempt: 1, generation: 1, plan });
    expect(store.getSnapshot().context.plan).toBeNull();

    store.trigger.dryRunRequested({ execute: () => new Promise<never>(() => undefined) });
    store.trigger.dryRunSucceeded({ attempt: 2, generation: 3, plan });
    expect(store.getSnapshot().context.plan).toEqual(plan);
  });

  it("Should replace the draft when the workspace-and-loop scope changes", () => {
    const { result, rerender } = renderHook(
      ({ scope }: { scope: FormScope }) => useLoopRunFormState(formInput(scope)),
      {
        initialProps: {
          scope: { workspaceId: "workspace-alpha", loopName: "review" },
        },
      }
    );

    act(() => {
      result.current.setInput("goal", "Edited goal");
    });

    rerender({
      scope: { workspaceId: "workspace-beta", loopName: "review" },
    });
    expect(result.current.inputs).toEqual({});
  });

  describe("run confirmation", () => {
    beforeEach(() => {
      notifyUser.mockClear();
    });

    function runStore() {
      return loopRunFormLogic.createStore({
        effectiveConfig: loopEffectiveConfigFixture,
        schema,
        scope: { loopName: "delivery", workspaceId: "workspace-alpha" },
      });
    }

    it("Should name the owner the daemon returned when the aggregate is on", async () => {
      const store = runStore();
      // The request acted as `default`; the daemon filed the run under
      // `marketing`. Echoing the request would hide exactly that.
      store.trigger.runRequested({
        announceOwner: true,
        execute: async () => ({ run: { id: "run_9", profile_name: "marketing" } }) as never,
        loopName: "delivery",
      });

      await waitFor(() =>
        expect(notifyUser).toHaveBeenCalledWith({
          message: "Run started · run_9. Created in marketing.",
          tone: "success",
        })
      );
    });

    it("Should claim no owner under a scoped view", async () => {
      const store = runStore();
      store.trigger.runRequested({
        announceOwner: false,
        execute: async () => ({ run: { id: "run_9", profile_name: "marketing" } }) as never,
        loopName: "delivery",
      });

      await waitFor(() =>
        expect(notifyUser).toHaveBeenCalledWith({ message: "Run started · run_9", tone: "success" })
      );
    });

    it("Should never claim an owner for a dry run", async () => {
      const store = runStore();
      // A dry run creates nothing, so the response carries no run and there is
      // no owner to name — saying otherwise would be a lie about durable state.
      store.trigger.dryRunRequested({ execute: async () => ({ dry_run: plan }) as never });

      await waitFor(() => expect(notifyUser).toHaveBeenCalled());
      const [[feedback]] = notifyUser.mock.calls;
      expect(feedback.message).toBe("Dry run passed — inputs valid, plan rendered");
      expect(feedback.message).not.toContain("Created in");
    });
  });
});
