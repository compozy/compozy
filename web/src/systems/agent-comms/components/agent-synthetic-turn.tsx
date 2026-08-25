/**
 * A daemon-authored turn, rendered in its own place in the transcript.
 *
 * Four shapes, all of them real turns the runtime wrote — the order is the
 * durable order, not a client-side merge:
 *
 * - **call-request / call-follow-up** — the ask a child received. This is the
 *   child's side of a delegation: it explains why the session is doing anything
 *   at all, which is otherwise the least obvious thing about a child session.
 * - **call-wake** — the completion that woke the caller, carrying the daemon's
 *   own summary line verbatim. Rephrasing it would let the screen and the
 *   agent's own context disagree about what happened.
 * - **message** — a note from another agent: provenance-stamped, framed, inert,
 *   and carrying its delivery receipt.
 */
import { Bell, CornerDownLeft, CornerUpRight } from "lucide-react";

import { Marker, MarkerMeta, MonoId, Time } from "@compozy/ui";

import { AgentCallStatePill } from "./agent-call-state-pill";
import { AgentMessageDeliveryPill } from "./agent-call-state-pill";
import { AgentUntrustedFrame } from "./agent-untrusted-frame";
import { toCallDelivery, toCallState } from "../lib/call-state";
import type { SyntheticTurn } from "../lib/synthetic-turn";

export interface AgentSyntheticTurnProps {
  turn: SyntheticTurn;
  /** The turn's own text, as written into the transcript. */
  text: string;
  timestamp?: string;
  onOpenCall?: (callId: string) => void;
  "data-testid"?: string;
}

function OpenCall({
  callId,
  onOpenCall,
}: {
  callId: string | null;
  onOpenCall?: (callId: string) => void;
}) {
  if (callId === null || !onOpenCall) return null;
  return (
    <button
      className="shrink-0 text-form text-accent underline-offset-4 hover:underline"
      onClick={() => onOpenCall(callId)}
      type="button"
    >
      Open call
    </button>
  );
}

export function AgentSyntheticTurn({
  turn,
  text,
  timestamp,
  onOpenCall,
  "data-testid": testId,
}: AgentSyntheticTurnProps) {
  if (turn.kind === "message") {
    const author = turn.childAgentName ?? turn.childSessionId ?? "another agent";
    return (
      <article
        className="flex flex-col gap-1.5"
        data-message-id={turn.messageId ?? undefined}
        data-synthetic-kind="message"
        data-testid={testId}
      >
        <div className="flex items-center gap-2">
          <span className="text-form text-fg">{author}</span>
          <AgentMessageDeliveryPill
            data-testid="agent-message-delivery"
            delivery={toCallDelivery(turn.deliveryKind ?? undefined)}
            fallbackLabel={turn.deliveryKind ?? undefined}
          />
          {/*
            A delivery that failed is only half the receipt. The daemon's own
            reason is the other half, rendered verbatim — "queued" and "failed"
            look alike to an operator until it says why.
          */}
          {turn.reason ? (
            <span
              className="font-mono text-form text-muted"
              data-testid="agent-message-failure-reason"
            >
              {turn.reason}
            </span>
          ) : null}
          <span className="flex-1" />
          {timestamp ? <Time className="text-form text-muted" iso={timestamp} /> : null}
        </div>
        <AgentUntrustedFrame authorLabel={author} sourceId={turn.childSessionId}>
          {text}
        </AgentUntrustedFrame>
      </article>
    );
  }

  if (turn.kind === "call-wake") {
    const state = toCallState(turn.callState ?? undefined);
    return (
      <aside
        className="rounded-md border border-line-soft bg-canvas-soft px-3 py-2"
        data-call-id={turn.callId ?? undefined}
        data-synthetic-kind="call-wake"
        data-testid={testId}
      >
        <p className="flex items-center gap-1.5 text-form text-muted">
          <Bell aria-hidden="true" className="size-3 shrink-0" />
          Woke because a call settled
          {state !== null ? <AgentCallStatePill state={state} /> : null}
        </p>
        {/*
          The daemon's own wake line, character for character. This is the text
          the agent itself received, so the operator reading the screen and the
          agent reading its context are looking at the same sentence.
        */}
        <pre className="mt-1.5 whitespace-pre-wrap break-words font-mono text-form text-fg">
          {turn.summary ?? text}
        </pre>
        <p className="mt-1.5 flex items-center gap-2 text-form text-muted">
          {turn.resultBytes !== null ? <span>{turn.resultBytes} B</span> : null}
          {turn.contractDigest ? <MonoId value={turn.contractDigest} /> : null}
          <span className="flex-1" />
          <OpenCall callId={turn.callId} onOpenCall={onOpenCall} />
        </p>
      </aside>
    );
  }

  // The ask a child received — the first thing that explains its existence.
  const follow = turn.kind === "call-follow-up";
  return (
    <div
      className="flex flex-col gap-1.5"
      data-call-id={turn.callId ?? undefined}
      data-synthetic-kind={turn.kind}
      data-testid={testId}
    >
      <Marker
        icon={follow ? <CornerDownLeft aria-hidden="true" /> : <CornerUpRight aria-hidden="true" />}
        tone="neutral"
      >
        <b>{follow ? "asked again" : "asked to help"}</b>{" "}
        <MarkerMeta>{turn.childAgentName ?? turn.childSessionId ?? "this session"}</MarkerMeta>
        {turn.contractDigest ? (
          <>
            {" "}
            <MarkerMeta>contract</MarkerMeta> <MonoId value={turn.contractDigest} />
          </>
        ) : null}
      </Marker>
      <p className="whitespace-pre-wrap break-words text-small-body text-fg">{text}</p>
      {turn.callId && onOpenCall ? (
        <p className="flex">
          <span className="flex-1" />
          <OpenCall callId={turn.callId} onOpenCall={onOpenCall} />
        </p>
      ) : null}
    </div>
  );
}
