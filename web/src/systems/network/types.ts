import type { OperationQuery, OperationRequestBody, OperationResponse } from "@/lib/api-contract";

export type NetworkSurface = "thread" | "direct";

export type NetworkSignalTone = "accent" | "success" | "warning" | "danger" | "info" | "neutral";

export type NetworkKindFilter =
  | "all"
  | "say"
  | "receipt"
  | "capability"
  | "greet"
  | "whois"
  | "trace";

export type NetworkStatusResponse = OperationResponse<"getNetworkStatus", 200>;
export type NetworkStatus = NetworkStatusResponse["network"];

type TaskRunDetailNetwork = OperationResponse<"getTaskRun", 200>["run"]["network"];
export type TaskRunNetworkProjection = NonNullable<TaskRunDetailNetwork>;
export type TaskRunNetworkUsage = TaskRunNetworkProjection["usage"];
export type NetworkUsageQuery = OperationQuery<"getNetworkUsage">;
export type NetworkUsageFilters = Omit<NetworkUsageQuery, "cursor">;

export type NetworkChannelsResponse = OperationResponse<"listNetworkChannels", 200>;
export type NetworkChannelSummary = NetworkChannelsResponse["channels"][number];
export type NetworkChannelsQuery = OperationQuery<"listNetworkChannels">;

export type NetworkChannelDetailResponse = OperationResponse<"getNetworkChannel", 200>;
export type NetworkChannel = NetworkChannelDetailResponse["channel"];
export type NetworkChannelUpdateRequest = OperationRequestBody<"updateNetworkChannel">;
export type NetworkChannelUpdateResponse = OperationResponse<"updateNetworkChannel", 200>;
export type NetworkFanoutPolicy = "capability_match" | "coordinator" | "all_members";

export type NetworkThreadsResponse = OperationResponse<"listNetworkThreads", 200>;
export type NetworkThreadSummary = NetworkThreadsResponse["threads"][number];
export type NetworkThreadsListQuery = OperationQuery<"listNetworkThreads">;

export type NetworkThreadDetailResponse = OperationResponse<"getNetworkThread", 200>;
export type NetworkThreadTaskLink = NonNullable<NetworkThreadDetailResponse["task_links"]>[number];
export type NetworkThreadDetail = NetworkThreadDetailResponse["thread"] & {
  task_links?: NetworkThreadTaskLink[];
};

export type NetworkThreadMessagesResponse = OperationResponse<"listNetworkThreadMessages", 200>;
export type NetworkThreadMessage = NetworkThreadMessagesResponse["messages"][number];

export type NetworkDirectRoomsResponse = OperationResponse<"listNetworkDirectRooms", 200>;
export type NetworkDirectRoomSummary = NetworkDirectRoomsResponse["directs"][number];
export type NetworkDirectsListQuery = OperationQuery<"listNetworkDirectRooms">;

export type NetworkDirectRoomDetailResponse = OperationResponse<"getNetworkDirectRoom", 200>;
export type NetworkDirectRoomDetail = NetworkDirectRoomDetailResponse["direct"];

export type NetworkDirectRoomMessagesResponse = OperationResponse<
  "listNetworkDirectRoomMessages",
  200
>;
export type NetworkDirectRoomMessage = NetworkDirectRoomMessagesResponse["messages"][number];

export type NetworkResolveDirectRoomRequest = OperationRequestBody<"resolveNetworkDirectRoom">;
export type NetworkResolveDirectRoomResponse = OperationResponse<"resolveNetworkDirectRoom", 200>;

export type NetworkSubscriptionsResponse = OperationResponse<"listNetworkSubscriptions", 200>;
export type NetworkSubscription = NetworkSubscriptionsResponse["subscriptions"][number];
export type NetworkSubscriptionRequest = OperationRequestBody<"upsertNetworkSubscription">;
export type NetworkSubscriptionResponse = OperationResponse<"upsertNetworkSubscription", 200>;
export type NetworkSubscriptionMode = "mute" | "full";

export type PromoteNetworkThreadTaskRequest = OperationRequestBody<"promoteNetworkThreadTask">;
export type PromoteNetworkThreadTaskResponse = OperationResponse<"promoteNetworkThreadTask", 201>;

export type NetworkWorkResponse = OperationResponse<"getNetworkWork", 200>;
export type NetworkWorkDetail = NetworkWorkResponse["work"];

export type NetworkConversationMessage = NetworkThreadMessage | NetworkDirectRoomMessage;

export type NetworkPeersResponse = OperationResponse<"listNetworkPeers", 200>;
export type NetworkPeerSummary = NetworkPeersResponse["peers"][number];

export type NetworkPeerDetailResponse = OperationResponse<"getNetworkPeer", 200>;
export type NetworkPeerDetail = NetworkPeerDetailResponse["peer"];

export type NetworkPeerCard = NetworkPeerSummary["peer_card"];
export type NetworkCapabilityBrief = NetworkPeerCard["capabilities"][number];

export type NetworkCapabilityCatalog = NonNullable<NetworkPeerDetail["capability_catalog"]>;
export type NetworkCapability = NetworkCapabilityCatalog["capabilities"][number];

export type NetworkPresenceState = "local";

export interface NetworkPresence {
  state: NetworkPresenceState;
}

export type CreateNetworkChannelRequest = OperationRequestBody<"createNetworkChannel">;
export type CreateNetworkChannelResponse = OperationResponse<"createNetworkChannel", 201>;
export type NetworkSendRequest = OperationRequestBody<"sendNetworkMessage">;
export type NetworkSendResponse = OperationResponse<"sendNetworkMessage", 200>;

export interface NetworkCreateChannelDraft {
  channelName: string;
  purpose: string;
  selectedAgentNames: string[];
  fanoutPolicy: NetworkFanoutPolicy;
}

export type NetworkConversationMessagesQuery = OperationQuery<"listNetworkThreadMessages">;
export type NetworkConversationMessageFilters = Omit<
  NetworkConversationMessagesQuery,
  "after" | "before"
>;
export type NetworkCursorPageParam = string | null;
export type NetworkMessagePageParam = { before: string } | null;
export type NetworkConversationMessagesResponse =
  | NetworkThreadMessagesResponse
  | NetworkDirectRoomMessagesResponse;

export type NetworkRouteSurface = NetworkSurface | "activity";

export interface NetworkRecentEntry {
  surface: NetworkSurface;
  channel: string;
  containerId: string;
  preview: string;
  lastActivityAt: string | null;
  hasUnread: boolean;
  participantLabel: string;
}
