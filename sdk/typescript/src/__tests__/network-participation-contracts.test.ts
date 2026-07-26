import { describe, expect, expectTypeOf, it } from "vitest";

import type {
  NetworkParticipationRequest,
  NetworkParticipationSpec,
} from "../generated/contracts.js";

describe("generated Network participation contracts", () => {
  it("accepts only runtime-valid authored variants", () => {
    const local = {} satisfies NetworkParticipationRequest;
    const named = {
      mode: "live",
      channel_strategy: "named",
      channel_id: "builders",
    } satisfies NetworkParticipationRequest;
    const run = {
      mode: "live",
      channel_strategy: "run",
    } satisfies NetworkParticipationRequest;

    expectTypeOf(local).toMatchTypeOf<NetworkParticipationRequest>();
    expectTypeOf(named).toMatchTypeOf<NetworkParticipationRequest>();
    expectTypeOf(run).toMatchTypeOf<NetworkParticipationRequest>();

    // @ts-expect-error Live participation requires a channel strategy.
    const missingStrategy: NetworkParticipationRequest = { mode: "live" };
    // @ts-expect-error Named participation requires a channel id.
    const missingNamedChannel: NetworkParticipationRequest = {
      mode: "live",
      channel_strategy: "named",
    };
    // @ts-expect-error Derived participation cannot carry a caller-authored channel id.
    const derivedChannel: NetworkParticipationRequest = {
      mode: "live",
      channel_strategy: "run",
      channel_id: "builders",
    };
    expect([missingStrategy, missingNamedChannel, derivedChannel]).toHaveLength(3);
  });

  it("accepts only complete resolved snapshots", () => {
    const local = {
      version: "network-participation/v1",
      mode: "local",
      source: "built_in_local",
    } satisfies NetworkParticipationSpec;
    const live = {
      version: "network-participation/v1",
      mode: "live",
      workspace_id: "ws-alpha",
      channel_strategy: "named",
      channel_id: "builders",
      source: "explicit_request",
      bounds: {
        max_wakes: 2,
        max_wake_wall_time: "5m",
        max_total_wall_time: "30m",
        max_input_tokens: 4096,
        max_output_tokens: 2048,
        max_wake_depth: 2,
        coalesce_window: "500ms",
      },
    } satisfies NetworkParticipationSpec;

    expectTypeOf(local).toMatchTypeOf<NetworkParticipationSpec>();
    expectTypeOf(live).toMatchTypeOf<NetworkParticipationSpec>();

    // @ts-expect-error Live snapshots require the resolved channel and finite bounds.
    const incompleteLive: NetworkParticipationSpec = {
      version: "network-participation/v1",
      mode: "live",
      workspace_id: "ws-alpha",
      channel_strategy: "run",
      source: "task_profile",
    };
    expect(incompleteLive.mode).toBe("live");
  });
});
