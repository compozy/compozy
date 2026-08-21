import { createClientId } from "@/lib/client-id";
import type { NetworkSendRequest } from "../types";
import type {
  OptimisticConversationMessage,
  SendNetworkMessageInput,
} from "./network-action-types";

export function generateClientMessageId(): string {
  return createClientId();
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

/**
 * Builds the local row shown before the daemon accepts the send.
 *
 * `owner` is the profile the message will be filed under once accepted. It is
 * passed in rather than derived here so the placeholder names the same profile
 * the destination chip promised. Until the profile list resolves the id stays
 * empty — the row is replaced by the labeled one on reconciliation either way.
 */
export function buildOptimisticMessage(
  input: SendNetworkMessageInput,
  clientMessageId: string,
  timestamp: string,
  owner: { id: string; name: string }
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
    profile_id: owner.id,
    profile_name: owner.name,
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
