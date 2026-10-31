import { useAuiState, type ThreadMessage } from "@assistant-ui/react";
import { use } from "react";

import { SessionSentCard } from "@/systems/session/components/session-messages/session-sent-card";
import type { SessionSentReplyState } from "@/systems/session/components/session-messages/session-sent-card";
import { SessionMessageReplyTo } from "@/systems/session/components/session-messages/session-message-reply-to";
import { useSessionLabel } from "@/systems/session/components/session-messages/use-session-label";
import { useSessionMessageOpen } from "@/systems/session/components/session-messages/use-session-message-open";
import { useSessionRuntimeRenderContext } from "@/systems/session/hooks/use-session-runtime-render-context";
import {
  sessionPromptMessageText,
  sessionReplyOutcomes,
  sessionSentCardOwners,
  sessionSentMessagePart,
  type SessionSentCallState,
  type SessionSentMessagePart,
} from "@/systems/session/lib/session-message-payload";
import {
  sessionMessageReplyIndex,
  sessionReplyKey,
} from "@/systems/session/lib/session-message-replies";
import type { SessionTimelineToolPart } from "@/systems/session/lib/session-timeline-parts";
import { SessionTranscriptMessagesContext } from "@/systems/session/lib/session-transcript-thread-context-value";
import { isRecord, stringField } from "@/systems/session/lib/timeline-message-parts";

import type { SessionSentMessageRow } from "./session-timeline-rows";

const NO_MESSAGES: readonly never[] = [];

function firstLine(text: string): string {
  return (
    text
      .split("\n")
      .map(line => line.trim())
      .find(line => line.length > 0) ?? ""
  );
}

function toolError(tool: SessionTimelineToolPart | null): string | null {
  if (!tool?.isError) return null;
  const result = tool.result;
  if (isRecord(result)) {
    const error = result.error;
    if (isRecord(error)) return stringField(error, "message")?.trim() || null;
    return stringField(result, "error")?.trim() || null;
  }
  return typeof result === "string" && result.trim() !== "" ? result.trim() : null;
}

// The call's own state wins over the part's when the call is loaded: a running
// call is still sending, an errored one could not send.
function callState(
  part: SessionSentMessagePart,
  tool: SessionTimelineToolPart | null
): SessionSentCallState {
  if (tool?.isError) return "failed";
  if (tool?.status === "running") return "sending";
  return part.state;
}

// An explicit answer (`in_reply_to`) reads Replied while the watch is still armed; the watch keeps
// running, so its outcome wins once the target's turn settles. An answer that arrived after the watch
// expired still reads Replied rather than No reply.
function sentReplyState(
  part: SessionSentMessagePart,
  messages: readonly ThreadMessage[]
): SessionSentReplyState {
  const outcome = part.replyWatchId
    ? sessionReplyOutcomes(messages).get(part.replyWatchId)
    : undefined;
  const key =
    part.messageId === null ? null : sessionReplyKey(part.targetSessionId, part.messageId);
  const answered = key !== null && sessionMessageReplyIndex(messages).answered.has(key);
  if (outcome && !(outcome === "expired" && answered)) return outcome;
  if (answered) return "completed";
  return part.replyWatchId ? "waiting" : "none";
}

/**
 * The "Sent to" card for one `compozy__session_prompt` call (S3). The reply
 * state is derived here: waiting until a reply wake with the same watch id is
 * in the loaded transcript, then that reply's outcome.
 */
export function SessionSentMessageRowView({ row }: { row: SessionSentMessageRow }) {
  const context = useSessionRuntimeRenderContext();
  const messageId = useAuiState(state => state.message.id);
  // Outside a thread provider (a story, a nested host) no reply is loaded: waiting.
  const messages = use(SessionTranscriptMessagesContext) ?? NO_MESSAGES;
  const onOpen = useSessionMessageOpen();
  const part = sessionSentMessagePart(row.part.name, row.part.data);
  const target = useSessionLabel({
    sessionId: part?.targetSessionId ?? "",
    workspaceId: part?.targetWorkspaceId ?? "",
    currentWorkspaceId: context?.workspaceId ?? "",
    agentName: null,
  });
  if (!part) return null;
  // One card per call: a later message that also holds its part draws it (#742).
  const owner = sessionSentCardOwners(messages).get(part.toolCallId);
  if (owner !== undefined && owner.messageId !== messageId) return null;
  const tool = row.toolPart;
  const message = sessionPromptMessageText(tool?.args) || (owner?.text ?? "");
  const reply = sentReplyState(part, messages);
  const timestamp = row.timestamp ? Date.parse(row.timestamp) : Number.NaN;
  return (
    <div data-testid="session-sent-message-row" className="flex min-w-0 flex-col py-1">
      <SessionSentCard
        target={target}
        callState={callState(part, tool)}
        mode={part.mode}
        firstLine={firstLine(message)}
        error={toolError(tool)}
        reply={reply}
        replyTo={
          part.inReplyTo ? (
            <SessionMessageReplyTo
              inReplyTo={part.inReplyTo}
              peerSessionId={part.targetSessionId}
            />
          ) : undefined
        }
        timestampMs={Number.isNaN(timestamp) ? null : timestamp}
        onOpenTarget={onOpen}
      />
    </div>
  );
}
