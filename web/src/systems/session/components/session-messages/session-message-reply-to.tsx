import { CornerUpLeft } from "@compozy/ui/icons";
import { use } from "react";

import { SessionMessageJumpContext } from "../../contexts/session-message-jump-context-value";
import {
  sessionMessageReplyIndex,
  sessionReplyTargetMessageId,
} from "../../lib/session-message-replies";
import { SessionTranscriptMessagesContext } from "../../lib/session-transcript-thread-context-value";
import { SessionMessageChip } from "./session-message-frame";

const LABEL = "In reply";
const NO_MESSAGES: readonly never[] = [];

/**
 * The `in_reply_to` correlation on a session-message card (#760): an "In
 * reply" chip that, when the original message is loaded in this transcript
 * and the viewport can scroll to it, takes the reader there.
 */
export function SessionMessageReplyTo({
  inReplyTo,
  peerSessionId,
}: {
  inReplyTo: string;
  /** The other session of the exchange: the answering sender, or the target this session answered. */
  peerSessionId: string;
}) {
  const jump = use(SessionMessageJumpContext);
  const messages = use(SessionTranscriptMessagesContext) ?? NO_MESSAGES;
  const targetMessageId = sessionReplyTargetMessageId(
    sessionMessageReplyIndex(messages),
    inReplyTo,
    peerSessionId
  );
  const chip = (
    <SessionMessageChip icon={<CornerUpLeft aria-hidden="true" />}>{LABEL}</SessionMessageChip>
  );
  if (targetMessageId === null || jump === null) {
    return (
      <span data-testid="session-message-reply-to" title="Answers an earlier message">
        {chip}
      </span>
    );
  }
  return (
    <button
      type="button"
      data-testid="session-message-reply-to"
      data-target-message-id={targetMessageId}
      aria-label="In reply: show the original message"
      title="Show the original message"
      onClick={() => jump(targetMessageId)}
      className="inline-flex shrink-0 rounded-full transition-opacity duration-base ease-out hover:opacity-80 focus-visible:shadow-focus-ring focus-visible:outline-none"
    >
      {chip}
    </button>
  );
}
