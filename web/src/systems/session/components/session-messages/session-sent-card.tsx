import { ArrowUpRight, CornerDownRight, Scissors } from "lucide-react";
import { useReducedMotionConfig } from "motion/react";

import type {
  SessionMessageMode,
  SessionReplyOutcome,
  SessionSentCallState,
} from "@/systems/session/lib/session-message-payload";
import { Spinner, StatusDot, cn, type StatusDotTone } from "@compozy/ui";

import {
  SessionMessageChip,
  SessionMessagePartyLabel,
  SessionMessageTime,
} from "./session-message-frame";
import {
  SESSION_MESSAGE_FRAME_CLASS,
  sessionPartyPhrase,
  type SessionMessageOpen,
  type SessionMessageParty,
} from "./session-message-party";

/** The sent card's right side: no state when no reply was asked for; waiting until the reply exists. */
export type SessionSentReplyState = "none" | "waiting" | SessionReplyOutcome;

const VERB: Record<SessionSentCallState, string> = {
  sending: "Sending to",
  sent: "Sent to",
  failed: "Could not send to",
};

const REPLY_WORD: Record<SessionReplyOutcome, string> = {
  completed: "Replied",
  failed: "Failed",
  canceled: "Canceled",
  dropped: "Dropped",
  unknown: "Unknown",
};

const REPLY_TONE: Record<SessionReplyOutcome, StatusDotTone> = {
  completed: "success",
  failed: "danger",
  canceled: "faint",
  dropped: "faint",
  unknown: "faint",
};

function SessionSentReply({
  callState,
  reply,
}: {
  callState: SessionSentCallState;
  reply: SessionSentReplyState;
}) {
  const reduced = useReducedMotionConfig();
  if (callState === "sending") {
    return (
      <span
        className="inline-flex items-center gap-1.5 text-transcript-caption whitespace-nowrap text-subtle"
        data-testid="session-sent-reply-state"
        data-state="sending"
      >
        <Spinner className="size-2.75" />
        Sending…
      </span>
    );
  }
  if (callState === "failed" || reply === "none") return null;
  const tone: StatusDotTone = reply === "waiting" ? "success" : REPLY_TONE[reply];
  const word = reply === "waiting" ? "Waiting for reply" : REPLY_WORD[reply];
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 text-transcript-caption whitespace-nowrap",
        tone === "success" ? "text-muted" : tone === "danger" ? "text-danger" : "text-subtle"
      )}
      data-testid="session-sent-reply-state"
      data-state={reply}
    >
      <StatusDot tone={tone} className={cn(reply === "waiting" && !reduced && "animate-pulse")} />
      {word}
    </span>
  );
}

export interface SessionSentCardProps {
  target: SessionMessageParty;
  callState: SessionSentCallState;
  mode: SessionMessageMode;
  /** The first line of the message; on a failed call, the error replaces it. */
  firstLine: string;
  error: string | null;
  reply: SessionSentReplyState;
  timestampMs: number | null;
  onOpenTarget?: SessionMessageOpen;
}

/**
 * A `compozy__session_prompt` call in the sender's transcript (S3, VC-04): one
 * row with the arrow-out tile, "Sent to {title}", the steer or interrupt chip,
 * the message's first line, and whether the target answered.
 */
export function SessionSentCard({
  target,
  callState,
  mode,
  firstLine,
  error,
  reply,
  timestampMs,
  onOpenTarget,
}: SessionSentCardProps) {
  const failed = callState === "failed";
  const verb = VERB[callState];
  const lineTwo = failed ? (error ?? firstLine) : firstLine;
  const replyWord =
    failed || callState === "sending" || reply === "none"
      ? null
      : reply === "waiting"
        ? "waiting for reply"
        : REPLY_WORD[reply].toLowerCase();
  const name = sessionPartyPhrase(verb, target);
  return (
    <article
      aria-label={replyWord ? `${name}, ${replyWord}` : name}
      data-testid="session-sent-card"
      data-state={callState}
      data-reply={reply}
      className={cn(
        SESSION_MESSAGE_FRAME_CLASS,
        "grid min-h-11.5 grid-cols-[24px_minmax(0,1fr)_auto] items-center gap-x-2.5 py-1.75 pr-3 pl-2.25"
      )}
    >
      <span
        aria-hidden="true"
        className={cn(
          "grid size-6 place-items-center rounded-full border bg-surface-2",
          failed ? "border-danger/40 text-danger" : "border-line-strong text-muted"
        )}
      >
        <ArrowUpRight className="size-3" />
      </span>
      <span className="flex min-w-0 flex-col gap-px">
        <span className="flex min-w-0 items-center gap-1.5">
          <SessionMessagePartyLabel verb={verb} party={target} onOpen={onOpenTarget} />
          {mode === "steer" ? (
            <SessionMessageChip icon={<CornerDownRight aria-hidden="true" />}>
              Steered
            </SessionMessageChip>
          ) : mode === "interrupt" ? (
            <SessionMessageChip icon={<Scissors aria-hidden="true" />}>
              Interrupted
            </SessionMessageChip>
          ) : null}
        </span>
        {lineTwo ? (
          <span
            className={cn(
              "min-w-0 truncate text-transcript-caption",
              failed ? "text-danger" : "text-muted"
            )}
            data-testid="session-sent-line-two"
          >
            {lineTwo}
          </span>
        ) : null}
      </span>
      <span className="inline-flex shrink-0 items-center gap-2.5">
        <SessionSentReply callState={callState} reply={reply} />
        <SessionMessageTime timestampMs={timestampMs} />
      </span>
    </article>
  );
}
