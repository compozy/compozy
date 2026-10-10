import { MessagePrimitive, useAuiState } from "@assistant-ui/react";

import { SessionAttachmentGallery } from "@/systems/session/components/session-attachment-gallery";
import { SessionMessageCard } from "@/systems/session/components/session-messages/session-message-card";
import type { SessionMessageDelivery } from "@/systems/session/components/session-messages/session-message-card";
import { SessionReplyCard } from "@/systems/session/components/session-messages/session-reply-card";
import { useSessionLabel } from "@/systems/session/components/session-messages/use-session-label";
import { useSessionMessageOpen } from "@/systems/session/components/session-messages/use-session-message-open";
import { useSessionRuntimeRenderContext } from "@/systems/session/hooks/use-session-runtime-render-context";
import { useSteerProvenance } from "@/systems/session/hooks/use-steer-provenance";
import {
  userMessageAttachmentItems,
  userMessageHasText,
} from "@/systems/session/lib/session-attachment-items";
import type {
  SessionMessageOrigin,
  SessionReplyMeta,
} from "@/systems/session/lib/session-message-payload";
import type { SteerMarkerKind } from "@/systems/session/lib/steer-marker";

import { deriveMessageActions } from "./message-actions.logic";
import { UserMessageParts } from "./session-user-message";

// Interrupt-and-replace reads "Interrupted"; a live or pending steer reads
// "Steered"; the queue reads nothing (Gap 3).
function deliveryFromSteer(kind: SteerMarkerKind | undefined): SessionMessageDelivery | null {
  switch (kind) {
    case "injected":
    case "pending_injection":
      return "steered";
    case "interrupt_fallback":
      return "interrupted";
    case "superseded":
      return "superseded";
    default:
      return null;
  }
}

function customTimestampMs(metadata: unknown): number | null {
  const custom = (metadata as { custom?: { timestamp?: unknown } } | undefined)?.custom;
  if (typeof custom?.timestamp !== "string") return null;
  const parsed = Date.parse(custom.timestamp);
  return Number.isNaN(parsed) ? null : parsed;
}

function messageTimestampMs(message: { content?: unknown; metadata?: unknown }): number | null {
  return customTimestampMs(message.metadata) ?? deriveMessageActions(message).timestampMs;
}

/**
 * A user turn another session sent (S1): the role stays `user` (scroll anchor,
 * status and estimates treat it as one); only the presentation is the
 * left-aligned card.
 */
export function SessionAgentMessage({ origin }: { origin: SessionMessageOrigin }) {
  const message = useAuiState(state => state.message);
  const context = useSessionRuntimeRenderContext();
  const workspaceId = context?.workspaceId ?? "";
  const sender = useSessionLabel({
    sessionId: origin.sessionId,
    workspaceId: origin.workspaceId,
    currentWorkspaceId: workspaceId,
    agentName: origin.agentName,
    titleAtSend: origin.titleAtSend,
  });
  const onOpen = useSessionMessageOpen();
  const steer = useSteerProvenance().forMessage(message);
  const attachments = userMessageAttachmentItems(
    message.content,
    message.metadata,
    workspaceId,
    context?.sessionId ?? ""
  );
  return (
    <MessagePrimitive.Root
      className="group/message flex w-full min-w-0 justify-start pt-1 pb-transcript-turn-gap"
      data-steer={steer?.kind}
      data-origin="session"
    >
      <SessionMessageCard
        sender={sender}
        delivery={deliveryFromSteer(steer?.kind)}
        replyRequested={origin.notifyOnComplete}
        timestampMs={messageTimestampMs(message)}
        onOpenSender={onOpen}
        attachments={
          attachments.length > 0 ? (
            <SessionAttachmentGallery items={attachments} className="mb-0 self-start" />
          ) : undefined
        }
      >
        {userMessageHasText(message.content) ? <UserMessageParts /> : null}
      </SessionMessageCard>
    </MessagePrimitive.Root>
  );
}

/** The reply wake on the sender's side (S2); every other synthetic kind renders nothing. */
export function SessionReplyMessage({ reply }: { reply: SessionReplyMeta }) {
  const message = useAuiState(state => state.message);
  const context = useSessionRuntimeRenderContext();
  const target = useSessionLabel({
    sessionId: reply.targetSessionId,
    workspaceId: reply.targetWorkspaceId,
    currentWorkspaceId: context?.workspaceId ?? "",
    agentName: reply.targetAgentName,
  });
  const onOpen = useSessionMessageOpen();
  return (
    <MessagePrimitive.Root
      className="flex w-full min-w-0 justify-start pt-1 pb-transcript-turn-gap"
      data-synthetic="session_reply"
    >
      <SessionReplyCard
        target={target}
        outcome={reply.outcome}
        text={reply.text}
        truncated={reply.truncated}
        timestampMs={messageTimestampMs(message)}
        onOpenTarget={onOpen}
      />
    </MessagePrimitive.Root>
  );
}
