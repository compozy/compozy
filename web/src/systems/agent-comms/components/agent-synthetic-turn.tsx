/**
 * A daemon-authored turn, rendered in its own place in the transcript.
 *
 * - **call-request / call-follow-up** — the child's bound plate: who asked.
 * - **call-return** — the child closed the call.
 * - **call-wake** — the completion that woke the caller, fact lines only.
 * - **message** — a note from another agent: provenance-stamped and inert.
 */
import { Bell } from "lucide-react";

import { Marker, Time } from "@compozy/ui";

import { AgentCallBoundTurn } from "./agent-call-bound-turn";
import { callerDisplayName } from "../lib/ask-gist";
import { AgentCallReturnTurn } from "./agent-call-return-turn";
import { AgentMessageDeliveryPill } from "./agent-call-state-pill";
import { AgentUntrustedFrame } from "./agent-untrusted-frame";
import { toCallDelivery } from "../lib/call-state";
import type { SyntheticTurn } from "../lib/synthetic-turn";
import { operatorWakePreview } from "../lib/wake-preview";

export interface AgentSyntheticTurnProps {
  turn: SyntheticTurn;
  /** The turn's own text, as written into the transcript. */
  text: string;
  timestamp?: string;
  onOpenCall?: (callId: string) => void;
  "data-testid"?: string;
}

export function AgentSyntheticTurn({
  turn,
  text,
  timestamp,
  "data-testid": testId,
}: AgentSyntheticTurnProps) {
  if (turn.kind === "message") {
    const author = turn.childAgentName ?? "another agent";
    return (
      <article
        className="flex flex-col gap-1.5"
        data-message-id={turn.messageId ?? undefined}
        data-synthetic-kind="message"
        data-testid={testId}
      >
        <div className="flex items-center gap-2">
          <span className="font-medium text-transcript-meta text-fg">{author}</span>
          <AgentMessageDeliveryPill
            data-testid="agent-message-delivery"
            delivery={toCallDelivery(turn.deliveryKind ?? undefined)}
            fallbackLabel={turn.deliveryKind ?? undefined}
          />
          {turn.reason ? (
            <span
              className="font-mono text-transcript-caption text-muted"
              data-testid="agent-message-failure-reason"
            >
              {turn.reason}
            </span>
          ) : null}
          <span className="flex-1" />
          {timestamp ? (
            <Time className="text-transcript-caption text-muted" iso={timestamp} mode="compact" />
          ) : null}
        </div>
        <AgentUntrustedFrame authorLabel={author}>{text}</AgentUntrustedFrame>
      </article>
    );
  }

  if (turn.kind === "call-wake") {
    const preview = operatorWakePreview(turn.summary ?? text);
    return (
      <div
        className="flex flex-col gap-1"
        data-call-id={turn.callId ?? undefined}
        data-synthetic-kind="call-wake"
        data-testid={testId}
      >
        <Marker icon={<Bell aria-hidden="true" />} tone="neutral">
          <b>Woke because a call completed</b>
        </Marker>
        {preview !== "" ? (
          <p className="whitespace-pre-wrap break-words px-1 text-transcript-body text-muted">
            {preview}
          </p>
        ) : null}
      </div>
    );
  }

  if (turn.kind === "call-return") {
    return (
      <AgentCallReturnTurn
        callerName={callerDisplayName(turn)}
        data-testid={testId}
        verdict={turn.verdict}
      />
    );
  }

  return <AgentCallBoundTurn data-testid={testId} text={text} turn={turn} />;
}
