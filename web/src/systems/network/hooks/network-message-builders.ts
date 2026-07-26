import type { NetworkSendRequest } from "../types";
import type {
  OptimisticConversationMessage,
  SendNetworkMessageInput,
} from "./network-action-types";

export function generateClientMessageId(): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  const random = () => Math.floor(Math.random() * 0xffffffff).toString(16);
  return `${random()}${random()}-${random()}-${random()}-${random()}-${random()}${random()}${random()}`;
}

export function buildSendRequest(
  input: SendNetworkMessageInput,
  clientMessageId: string,
  workspaceId: string
): NetworkSendRequest {
  const base: NetworkSendRequest = {
    body: { text: input.text },
    channel: input.channel,
    id: clientMessageId,
    kind: "say",
    session_id: input.sessionId,
    surface: input.surface,
    workspace_id: workspaceId,
  };
  if (input.mentions?.length) base.mentions = input.mentions;
  if (input.surface === "thread") return { ...base, thread_id: input.threadId };
  return { ...base, direct_id: input.directId };
}

export function buildOptimisticMessage(
  input: SendNetworkMessageInput,
  clientMessageId: string,
  timestamp: string
): OptimisticConversationMessage {
  const base: OptimisticConversationMessage = {
    body: { text: input.text },
    channel: input.channel,
    direction: "sent",
    display_name: input.displayName,
    kind: "say",
    local: true,
    message_id: clientMessageId,
    optimistic: "pending",
    peer_from: input.peerFrom,
    preview_text: input.text,
    session_id: input.sessionId,
    surface: input.surface,
    text: input.text,
    timestamp,
  };
  if (input.mentions?.length) base.mentions = input.mentions;
  if (input.surface === "thread") base.thread_id = input.threadId;
  else {
    base.direct_id = input.directId;
    if (input.peerTo) base.peer_to = input.peerTo;
  }
  return base;
}
