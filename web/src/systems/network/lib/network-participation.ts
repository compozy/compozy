import type { OperationRequestBody } from "@/lib/api-contract";

/** Public create/edit participation request shape (UT-060). */
export type NetworkParticipationMode = "local" | "live";

export const NETWORK_PARTICIPATION_STRATEGIES = ["named", "run", "loop_run"] as const;

export type NetworkParticipationStrategy = (typeof NETWORK_PARTICIPATION_STRATEGIES)[number];

interface LocalNetworkParticipationDraft {
  mode: "local";
  channelId: string;
  channelStrategy: "";
}

interface LiveNetworkParticipationDraft {
  mode: "live";
  channelId: string;
  channelStrategy: NetworkParticipationStrategy;
}

export type NetworkParticipationDraft =
  | LocalNetworkParticipationDraft
  | LiveNetworkParticipationDraft;

export type NetworkParticipationPayload = NonNullable<
  OperationRequestBody<"createSession">["network_participation"]
>;

export const DEFAULT_NETWORK_PARTICIPATION_DRAFT: NetworkParticipationDraft = {
  mode: "local",
  channelId: "",
  channelStrategy: "",
};

export function isNetworkParticipationStrategy(
  value: string | null | undefined
): value is NetworkParticipationStrategy {
  return NETWORK_PARTICIPATION_STRATEGIES.some(strategy => strategy === value);
}

export function networkParticipationDraftFromValues(
  mode: string | null | undefined,
  channelId: string | null | undefined,
  channelStrategy: string | null | undefined
): NetworkParticipationDraft {
  if (mode !== "live") {
    return DEFAULT_NETWORK_PARTICIPATION_DRAFT;
  }
  const strategy = isNetworkParticipationStrategy(channelStrategy) ? channelStrategy : "named";
  return {
    mode: "live",
    channelId: strategy === "named" ? (channelId ?? "") : "",
    channelStrategy: strategy,
  };
}

/** Hydrate the shared control from an optional public participation request. */
export function networkParticipationDraftFromPayload(
  payload?: {
    mode?: string | null;
    channel_id?: string | null;
    channel_strategy?: string | null;
  } | null
): NetworkParticipationDraft {
  return networkParticipationDraftFromValues(
    payload?.mode,
    payload?.channel_id,
    payload?.channel_strategy
  );
}

export function networkParticipationValidationMessage(
  draft: NetworkParticipationDraft,
  allowedStrategies: readonly NetworkParticipationStrategy[] = NETWORK_PARTICIPATION_STRATEGIES
): string | null {
  if (draft.mode === "local") {
    return null;
  }
  if (!allowedStrategies.includes(draft.channelStrategy)) {
    return `Channel strategy ${draft.channelStrategy} is not available for this execution.`;
  }
  if (draft.channelStrategy === "named" && draft.channelId.trim() === "") {
    return "Channel is required for named participation.";
  }
  return null;
}

export function isNetworkParticipationDraftValid(
  draft: NetworkParticipationDraft,
  allowedStrategies?: readonly NetworkParticipationStrategy[]
): boolean {
  return networkParticipationValidationMessage(draft, allowedStrategies) === null;
}

/**
 * Serialize draft to exact `network_participation` contract.
 * Local default omits channel fields; never emits legacy participation keys.
 */
export function serializeNetworkParticipation(
  draft: NetworkParticipationDraft
): NetworkParticipationPayload {
  if (draft.mode === "local") {
    return { mode: "local" };
  }
  const channelId = draft.channelId.trim();
  const channelStrategy = draft.channelStrategy;
  if (channelStrategy === "named") {
    return {
      mode: "live",
      channel_id: channelId,
      channel_strategy: "named",
    };
  }
  return {
    mode: "live",
    channel_strategy: channelStrategy,
  };
}
