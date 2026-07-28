import type {
  NetworkChannelUpdateRequest,
  NetworkConversationMessage,
  NetworkResolveDirectRoomRequest,
  NetworkSendResponse,
  NetworkSubscriptionRequest,
  PromoteNetworkThreadTaskRequest,
} from "../types";

export interface OptimisticMessageMeta {
  optimistic: "pending" | "failed";
}

export type OptimisticConversationMessage = NetworkConversationMessage & OptimisticMessageMeta;

export interface SendNetworkMessageThreadInput {
  surface: "thread";
  channel: string;
  threadId: string;
  sessionId: string;
  text: string;
  mentions?: string[];
  peerFrom: string;
  displayName?: string;
  clientMessageId?: string;
}

export interface SendNetworkMessageDirectInput {
  surface: "direct";
  channel: string;
  directId: string;
  sessionId: string;
  text: string;
  mentions?: string[];
  peerFrom: string;
  peerTo?: string;
  displayName?: string;
  clientMessageId?: string;
}

export type SendNetworkMessageInput = SendNetworkMessageThreadInput | SendNetworkMessageDirectInput;
export type ScopedSendNetworkMessageInput = SendNetworkMessageInput & { workspaceId: string };

export interface SendNetworkMessageResult {
  clientMessageId: string;
  response: NetworkSendResponse;
}

export interface UseSendNetworkMessageResult {
  send: (input: SendNetworkMessageInput) => Promise<SendNetworkMessageResult>;
  retry: (
    input: SendNetworkMessageInput,
    clientMessageId: string
  ) => Promise<SendNetworkMessageResult>;
  discard: (input: SendNetworkMessageInput, clientMessageId: string) => void;
  isSending: boolean;
}

export interface CreateNetworkThreadInput {
  channel: string;
  sessionId: string;
  text: string;
  mentions?: string[];
  peerFrom: string;
  displayName?: string;
}

export interface CreateNetworkThreadResult {
  threadId: string;
  rootMessageId: string;
}

export interface UseCreateNetworkThreadResult {
  createThread: (input: CreateNetworkThreadInput) => Promise<CreateNetworkThreadResult>;
  isCreating: boolean;
}

export interface UseCreateNetworkThreadOptions {
  workspaceId?: string | null;
}

export interface ResolveNetworkDirectRoomInput {
  channel: string;
  body: NetworkResolveDirectRoomRequest;
}

export interface UpdateNetworkChannelInput {
  workspaceId: string;
  channel: string;
  data: NetworkChannelUpdateRequest;
}

export interface UpsertNetworkSubscriptionInput {
  workspaceId: string;
  channel: string;
  data: NetworkSubscriptionRequest;
}

export interface DeleteNetworkSubscriptionInput {
  workspaceId: string;
  channel: string;
  sessionId: string;
  threadId?: string;
}

export interface PromoteNetworkThreadTaskInput {
  workspaceId: string;
  channel: string;
  threadId: string;
  data: PromoteNetworkThreadTaskRequest;
}
