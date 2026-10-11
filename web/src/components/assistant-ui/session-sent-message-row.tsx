import { use } from "react";

import { SessionSentCard } from "@/systems/session/components/session-messages/session-sent-card";
import type { SessionSentReplyState } from "@/systems/session/components/session-messages/session-sent-card";
import { useSessionLabel } from "@/systems/session/components/session-messages/use-session-label";
import { useSessionMessageOpen } from "@/systems/session/components/session-messages/use-session-message-open";
import { useSessionRuntimeRenderContext } from "@/systems/session/hooks/use-session-runtime-render-context";
import {
  sessionReplyOutcomes,
  sessionSentMessagePart,
  type SessionSentCallState,
  type SessionSentMessagePart,
} from "@/systems/session/lib/session-message-payload";
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
  if (isRecord(result)) return stringField(result, "error")?.trim() || null;
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

/**
 * The "Sent to" card for one `compozy__session_prompt` call (S3). The reply
 * state is derived here: waiting until a reply wake with the same watch id is
 * in the loaded transcript, then that reply's outcome.
 */
export function SessionSentMessageRowView({ row }: { row: SessionSentMessageRow }) {
  const context = useSessionRuntimeRenderContext();
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
  const tool = row.toolPart;
  const message = tool ? (stringField(tool.args, "message") ?? "") : "";
  const outcome = part.replyWatchId
    ? sessionReplyOutcomes(messages).get(part.replyWatchId)
    : undefined;
  const reply: SessionSentReplyState = part.replyWatchId ? (outcome ?? "waiting") : "none";
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
        timestampMs={Number.isNaN(timestamp) ? null : timestamp}
        onOpenTarget={onOpen}
      />
    </div>
  );
}
