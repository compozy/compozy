import type { OperationResponse } from "@/lib/api-contract";

type SessionPayload = OperationResponse<"listSessions", 200>["sessions"][number];

export type ResolvedNetworkParticipationFixture = NonNullable<
  SessionPayload["resolved_network_participation"]
>;

type LocalNetworkParticipationFixture = Extract<
  ResolvedNetworkParticipationFixture,
  { mode: "local" }
>;
type LiveNetworkParticipationFixture = Extract<
  ResolvedNetworkParticipationFixture,
  { mode: "live" }
>;
type NetworkParticipationBoundsFixture = LiveNetworkParticipationFixture["bounds"];

const LIVE_BOUNDS: NetworkParticipationBoundsFixture = {
  coalesce_window: "500ms",
  max_input_tokens: 200_000,
  max_output_tokens: 50_000,
  max_total_wall_time: "30m",
  max_wake_depth: 3,
  max_wake_wall_time: "5m",
  max_wakes: 8,
};

export function buildLocalNetworkParticipationFixture(): LocalNetworkParticipationFixture {
  return {
    version: "network-participation/v1",
    mode: "local",
    source: "built_in_local",
  };
}

export function buildLiveNetworkParticipationFixture(options: {
  workspaceId: string;
  channelId: string;
  channelStrategy?: "named" | "run" | "loop_run";
  source?: Exclude<LiveNetworkParticipationFixture["source"], "built_in_local">;
}): LiveNetworkParticipationFixture {
  return {
    version: "network-participation/v1",
    mode: "live",
    source: options.source ?? "explicit_request",
    workspace_id: options.workspaceId,
    channel_strategy: options.channelStrategy ?? "named",
    channel_id: options.channelId,
    bounds: { ...LIVE_BOUNDS },
  };
}
