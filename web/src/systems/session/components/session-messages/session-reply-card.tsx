import { Scissors } from "lucide-react";

import type { SessionReplyOutcome } from "@/systems/session/lib/session-message-payload";
import { Button, cn } from "@compozy/ui";

import { MessageMarkdown } from "../message-markdown";
import { SubagentAvatar } from "../subagents/subagent-avatar";
import type { SubagentStatus } from "../subagents/types";
import {
  SessionMessageFrame,
  SessionMessagePartyLabel,
  type SessionMessageBodyTone,
} from "./session-message-frame";
import {
  sessionPartyPhrase,
  type SessionMessageOpen,
  type SessionMessageParty,
} from "./session-message-party";
import { SESSION_REPLY_OUTCOME_WORD } from "./session-reply-outcome";

// The outcome dot mirrors the subagent mapping: success · danger · faint.
const OUTCOME_DOT: Record<SessionReplyOutcome, SubagentStatus> = {
  completed: "completed",
  failed: "failed",
  canceled: "canceled",
  dropped: "canceled",
  unknown: "canceled",
};

const FIXED_BODY: Partial<Record<SessionReplyOutcome, string>> = {
  dropped: "The message was removed from the queue before it ran.",
  unknown: "The message may not have been delivered; check the target session.",
};

const NO_REPLY_TEXT = "No reply text.";

export interface SessionReplyCardProps {
  target: SessionMessageParty;
  outcome: SessionReplyOutcome;
  /** The target's answer, or the error summary when it failed; `""` when the turn left none. */
  text: string;
  /** The answer was cut at 12,000 characters. */
  truncated: boolean;
  timestampMs: number | null;
  onOpenTarget?: SessionMessageOpen;
}

type ReplyBody = { kind: "fixed"; text: string } | { kind: "answer"; text: string };

function replyBody(outcome: SessionReplyOutcome, text: string): ReplyBody {
  const fixed = FIXED_BODY[outcome];
  if (fixed) return { kind: "fixed", text: fixed };
  if (text.trim() === "") return { kind: "fixed", text: NO_REPLY_TEXT };
  return { kind: "answer", text };
}

function bodyTone(outcome: SessionReplyOutcome, body: ReplyBody): SessionMessageBodyTone {
  if (body.kind === "fixed") return "quiet";
  return outcome === "failed" ? "danger" : "default";
}

/**
 * The answer that came back to the sender (S2, VC-03): the target's mark with
 * the outcome dot, "Reply from {title}", the outcome word, and the answer.
 * Identity and outcome are the daemon's typed fields; the body is text only.
 */
export function SessionReplyCard({
  target,
  outcome,
  text,
  truncated,
  timestampMs,
  onOpenTarget,
}: SessionReplyCardProps) {
  const word = SESSION_REPLY_OUTCOME_WORD[outcome];
  const body = replyBody(outcome, text);
  const name = `${sessionPartyPhrase("Reply from", target)}, ${word.toLowerCase()}`;
  return (
    <SessionMessageFrame
      aria-label={truncated ? `${name}, truncated` : name}
      aria-busy={target.pending || undefined}
      data-testid="session-reply-card"
      data-outcome={outcome}
      avatar={
        <SubagentAvatar provider={target.agentName} status={OUTCOME_DOT[outcome]} surface="rail" />
      }
      header={
        <>
          <SessionMessagePartyLabel verb="Reply from" party={target} onOpen={onOpenTarget} />
          <span
            className={cn(
              "shrink-0 text-transcript-caption",
              outcome === "failed" ? "text-danger" : "text-subtle"
            )}
            data-testid="session-reply-outcome"
          >
            {word}
          </span>
        </>
      }
      timestampMs={timestampMs}
      bodyTone={bodyTone(outcome, body)}
      footer={
        truncated ? (
          <span
            className="mt-1 flex items-center gap-1.5 border-t border-line-soft pt-1.75 text-transcript-caption text-subtle"
            data-testid="session-reply-truncated"
          >
            <Scissors aria-hidden="true" className="size-3 text-faint" />
            Reply truncated
            {target.title !== null && onOpenTarget ? (
              <>
                <span aria-hidden="true">·</span>
                <Button
                  type="button"
                  variant="link"
                  size="xs"
                  onClick={event =>
                    onOpenTarget(target, { newWindow: event.metaKey || event.ctrlKey })
                  }
                  className="h-auto px-0 text-transcript-caption text-fg-2"
                  data-testid="session-reply-read-full"
                >
                  Read the full turn
                </Button>
              </>
            ) : null}
          </span>
        ) : null
      }
    >
      {body.kind === "answer" && outcome !== "failed" ? (
        <MessageMarkdown content={body.text} compact />
      ) : (
        <p className="whitespace-pre-wrap">{body.text}</p>
      )}
    </SessionMessageFrame>
  );
}
