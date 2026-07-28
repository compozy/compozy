import type {
  NetworkChannelsQuery,
  NetworkConversationMessageFilters,
  NetworkDirectsListQuery,
  NetworkSurface,
  NetworkThreadsListQuery,
  NetworkUsageFilters,
} from "../types";
import type { NetworkCoordinationRef } from "../adapters/network-coordination-api";

function normalizeText(value?: string | null) {
  return value?.trim() ?? "";
}

function normalizeLimit(value?: number | null) {
  return value ?? 0;
}

function catalogQuerySegments(query?: NetworkThreadsListQuery | NetworkDirectsListQuery) {
  return [
    normalizeText(query?.query),
    normalizeText(query?.session_id),
    query?.has_work ?? null,
    normalizeText(query?.sort),
    normalizeLimit(query?.limit),
  ] as const;
}

function messageFilterSegments(query?: NetworkConversationMessageFilters) {
  return [
    normalizeText(query?.kind),
    normalizeText(query?.work_id),
    normalizeLimit(query?.limit),
  ] as const;
}

function usageFilterSegments(filters?: NetworkUsageFilters) {
  return [
    normalizeText(filters?.owner_kind),
    normalizeText(filters?.owner_id),
    normalizeText(filters?.run_id),
    normalizeText(filters?.channel),
    normalizeLimit(filters?.limit),
  ] as const;
}

export const networkKeys = {
  all: ["network"] as const,
  status: () => [...networkKeys.all, "status"] as const,
  workspace: (workspaceId: string) =>
    [...networkKeys.all, "workspace", normalizeText(workspaceId)] as const,

  channelsRoot: (workspaceId: string) =>
    [...networkKeys.workspace(workspaceId), "channels"] as const,
  channels: (workspaceId: string, query?: NetworkChannelsQuery) =>
    [
      ...networkKeys.channelsRoot(workspaceId),
      "list",
      normalizeLimit(query?.recent_limit),
    ] as const,
  channelDetails: (workspaceId: string) =>
    [...networkKeys.channelsRoot(workspaceId), "detail"] as const,
  channelDetail: (workspaceId: string, channel: string) =>
    [...networkKeys.channelDetails(workspaceId), normalizeText(channel)] as const,

  channelScope: (workspaceId: string, channel: string) =>
    [...networkKeys.workspace(workspaceId), "channel", normalizeText(channel)] as const,
  subscriptionsRoot: (workspaceId: string, channel: string) =>
    [...networkKeys.channelScope(workspaceId, channel), "subscriptions"] as const,
  subscriptions: (
    workspaceId: string,
    channel: string,
    query?: { session_id?: string | null; thread_id?: string | null }
  ) =>
    [
      ...networkKeys.subscriptionsRoot(workspaceId, channel),
      normalizeText(query?.session_id),
      normalizeText(query?.thread_id),
    ] as const,

  threadsRoot: (workspaceId: string, channel: string) =>
    [
      ...networkKeys.channelScope(workspaceId, channel),
      "thread" satisfies NetworkSurface,
      "list",
    ] as const,
  threadsList: (workspaceId: string, channel: string, query?: NetworkThreadsListQuery) =>
    [...networkKeys.threadsRoot(workspaceId, channel), ...catalogQuerySegments(query)] as const,
  threadDetail: (workspaceId: string, channel: string, threadId: string) =>
    [
      ...networkKeys.channelScope(workspaceId, channel),
      "thread" satisfies NetworkSurface,
      "detail",
      normalizeText(threadId),
    ] as const,
  threadMessagesRoot: (workspaceId: string, channel: string, threadId: string) =>
    [
      ...networkKeys.channelScope(workspaceId, channel),
      "thread" satisfies NetworkSurface,
      "messages",
      normalizeText(threadId),
    ] as const,
  threadMessages: (
    workspaceId: string,
    channel: string,
    threadId: string,
    query?: NetworkConversationMessageFilters
  ) =>
    [
      ...networkKeys.threadMessagesRoot(workspaceId, channel, threadId),
      ...messageFilterSegments(query),
    ] as const,
  threadMessageTail: (
    workspaceId: string,
    channel: string,
    threadId: string,
    query?: NetworkConversationMessageFilters
  ) =>
    [
      ...networkKeys.channelScope(workspaceId, channel),
      "message-tail",
      "thread" satisfies NetworkSurface,
      normalizeText(threadId),
      ...messageFilterSegments(query),
    ] as const,

  directsRoot: (workspaceId: string, channel: string) =>
    [
      ...networkKeys.channelScope(workspaceId, channel),
      "direct" satisfies NetworkSurface,
      "list",
    ] as const,
  directsList: (workspaceId: string, channel: string, query?: NetworkDirectsListQuery) =>
    [...networkKeys.directsRoot(workspaceId, channel), ...catalogQuerySegments(query)] as const,
  directDetail: (workspaceId: string, channel: string, directId: string) =>
    [
      ...networkKeys.channelScope(workspaceId, channel),
      "direct" satisfies NetworkSurface,
      "detail",
      normalizeText(directId),
    ] as const,
  directMessagesRoot: (workspaceId: string, channel: string, directId: string) =>
    [
      ...networkKeys.channelScope(workspaceId, channel),
      "direct" satisfies NetworkSurface,
      "messages",
      normalizeText(directId),
    ] as const,
  directMessages: (
    workspaceId: string,
    channel: string,
    directId: string,
    query?: NetworkConversationMessageFilters
  ) =>
    [
      ...networkKeys.directMessagesRoot(workspaceId, channel, directId),
      ...messageFilterSegments(query),
    ] as const,
  directMessageTail: (
    workspaceId: string,
    channel: string,
    directId: string,
    query?: NetworkConversationMessageFilters
  ) =>
    [
      ...networkKeys.channelScope(workspaceId, channel),
      "message-tail",
      "direct" satisfies NetworkSurface,
      normalizeText(directId),
      ...messageFilterSegments(query),
    ] as const,

  workRoot: (workspaceId: string) => [...networkKeys.workspace(workspaceId), "work"] as const,
  work: (workspaceId: string, workId: string) =>
    [...networkKeys.workRoot(workspaceId), normalizeText(workId)] as const,
  peersRoot: (workspaceId: string) => [...networkKeys.workspace(workspaceId), "peers"] as const,
  peers: (workspaceId: string, channel?: string | null) =>
    [...networkKeys.peersRoot(workspaceId), normalizeText(channel)] as const,
  peerDetails: (workspaceId: string) => [...networkKeys.peersRoot(workspaceId), "detail"] as const,
  peerDetail: (workspaceId: string, peerId: string) =>
    [...networkKeys.peerDetails(workspaceId), normalizeText(peerId)] as const,

  coordinationRoot: (workspaceId: string) =>
    [...networkKeys.workspace(workspaceId), "coordination"] as const,
  coordination: (workspaceId: string, ref: NetworkCoordinationRef) =>
    [
      ...networkKeys.coordinationRoot(workspaceId),
      ref.scope,
      normalizeText(ref.taskId),
      normalizeText(ref.runId),
    ] as const,
  usage: (workspaceId: string, filters?: NetworkUsageFilters) =>
    [...networkKeys.workspace(workspaceId), "usage", ...usageFilterSegments(filters)] as const,
};
