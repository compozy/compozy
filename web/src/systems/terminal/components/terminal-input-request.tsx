"use client";

import { Check, Clock, KeyRound, Keyboard, MessageCircleQuestionMark, X } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useEffect, useId, useState } from "react";

import {
  Button,
  cn,
  MonoId,
  Questionnaire,
  QuestionnaireDescription,
  QuestionnaireInput,
  QuestionnaireItem,
  QuestionnaireSubmit,
  QuestionnaireTitle,
  Time,
} from "@compozy/ui";

import { terminalInputOutcomeCopy } from "../lib/terminal-copy";
import { terminalInputExpiry } from "../lib/terminal-input-expiry";
import type { TerminalInputOutcome, TerminalInputRequest } from "../types";

export interface TerminalInputRequestCardProps {
  request: TerminalInputRequest;
  /** False for a watcher: answering takes control first, in one gesture. */
  canAnswerDirectly: boolean;
  /**
   * Who is asking, when a surface actually knows.
   *
   * `TerminalInputRequest` carries no requester, and the terminal's current
   * controller is not one: a takeover between the ask and the render would
   * attribute the question to whoever holds the lease now. Production omits
   * this until the daemon names the requester; the design boards supply it.
   */
  askedBy?: string;
  /** Names the terminal when several are asking at once. */
  showOrigin?: boolean;
  terminalTitle?: string;
  onAnswer: (input: string) => void;
  onReject: () => void;
  /** Fixes the clock. Tests and capture harnesses only. */
  now?: number;
}

const INPUT_FIELD_NAME = "terminal-input";
const EXPIRY_REFRESH_MS = 30_000;

/**
 * The agent's question, pinned to the terminal it belongs to.
 *
 * A redacted answer never renders, is never stored, and never reaches the
 * agent: the field masks it, and the only thing that survives anywhere is the
 * fact that hidden input of some length happened.
 *
 * The `Questionnaire` root is a real form and its field stays uncontrolled: a
 * controlled one would put the secret in React state and in the element's
 * serialized `value` attribute, where a devtools snapshot or an error report
 * could carry it off; here it exists only in the live DOM node until the form
 * is submitted and reset.
 */
export function TerminalInputRequestCard({
  request,
  canAnswerDirectly,
  askedBy,
  showOrigin = false,
  terminalTitle,
  onAnswer,
  onReject,
  now,
}: TerminalInputRequestCardProps) {
  const titleId = useId();
  const Glyph = request.redacted ? KeyRound : MessageCircleQuestionMark;
  // The title names who is waiting and what for — never the whole reason, which
  // reads in full one line below and would truncate to nothing up here.
  const title = askedBy
    ? `${askedBy} needs ${request.redacted ? "a password" : "an answer"}`
    : request.redacted
      ? "A password is needed"
      : "An answer is needed";
  const [liveNow, setLiveNow] = useState(() => now ?? Date.now());
  useEffect(() => {
    if (now !== undefined) return undefined;
    const timer = window.setInterval(() => setLiveNow(Date.now()), EXPIRY_REFRESH_MS);
    return () => window.clearInterval(timer);
  }, [now]);
  const expiry = terminalInputExpiry(request.requested_at, now ?? liveNow);
  return (
    <Questionnaire
      className="gap-2.5 border-line border-t bg-canvas px-3.5 py-3"
      data-redacted={request.redacted ? "true" : undefined}
      data-testid={`terminal-input-request-${request.id}`}
      onSubmit={event => {
        event.preventDefault();
        const form = event.currentTarget;
        const answer = new FormData(form).get(INPUT_FIELD_NAME);
        onAnswer(typeof answer === "string" ? answer : "");
        form.reset();
      }}
    >
      <QuestionnaireItem className="gap-2.5" name={INPUT_FIELD_NAME}>
        <QuestionnaireTitle className="flex w-full min-w-0 items-center gap-2">
          <span className="grid size-6.5 flex-none place-items-center rounded-sm bg-warning-tint text-warning">
            <Glyph aria-hidden="true" className="size-3.5" />
          </span>
          <span className="min-w-0 truncate" id={titleId}>
            {title}
          </span>
          {showOrigin && terminalTitle ? (
            <span className="flex flex-none items-center gap-1.5 font-mono font-normal text-micro text-subtle">
              {terminalTitle}
              <MonoId size="sm" value={request.terminal_id} />
            </span>
          ) : null}
          <span
            className={cn(
              "ml-auto flex-none font-mono font-normal text-micro",
              expiry.expired ? "text-muted" : "text-faint"
            )}
            data-testid={`terminal-input-request-expiry-${request.id}`}
          >
            {expiry.label}
          </span>
        </QuestionnaireTitle>
        <QuestionnaireDescription>{request.reason}</QuestionnaireDescription>
        <div className="rounded-xs bg-chat-fill-code px-2.25 py-1.5 font-mono text-badge leading-normal break-all whitespace-pre-wrap text-fg">
          {request.prompt_excerpt}
        </div>
        <div className="flex items-center gap-2">
          <div className="min-w-0 flex-1">
            <QuestionnaireInput
              // The title carries whose question this is; the label carries
              // what the field does (and, when redacted, the protection rule).
              aria-describedby={titleId}
              aria-label={
                request.redacted
                  ? "Hidden input — never shown, stored, or sent to the agent"
                  : "Your answer"
              }
              autoComplete="off"
              className={cn(request.redacted && "font-mono tracking-redacted")}
              data-testid={`terminal-input-request-field-${request.id}`}
              type={request.redacted ? "password" : "text"}
            />
          </div>
          {/* Neutral by the accent budget: while a watcher can see this card
              the head is already offering the accent Take control. */}
          <QuestionnaireSubmit
            data-testid={`terminal-input-request-send-${request.id}`}
            variant="neutral"
          >
            {canAnswerDirectly ? "Send" : "Take control & send"}
          </QuestionnaireSubmit>
          <Button
            data-testid={`terminal-input-request-decline-${request.id}`}
            onClick={onReject}
            size="sm"
            type="button"
            variant="ghost"
          >
            Decline
          </Button>
        </div>
      </QuestionnaireItem>
    </Questionnaire>
  );
}

const RESOLVED_GLYPHS: Record<TerminalInputOutcome, LucideIcon> = {
  answered: Check,
  rejected: X,
  superseded: Keyboard,
  expired: Clock,
};

const RESOLVED_TONES: Record<TerminalInputOutcome, string> = {
  answered: "bg-success-tint text-success",
  rejected: "bg-badge-fill text-muted",
  superseded: "bg-info-tint text-info",
  expired: "bg-badge-fill text-muted",
};

export interface TerminalInputResolvedRowProps {
  outcome: TerminalInputOutcome;
  /** Length only. The characters never existed outside the program. */
  redactedLength?: number;
  /** Who took over, on `superseded`. */
  supersededBy?: string;
  resolvedAt: string;
}

/**
 * A resolved question, collapsed to one quiet line.
 *
 * An answered redacted prompt states its length and nothing else — that marker
 * is the whole record, in the stream, the journal, and the replay alike.
 */
export function TerminalInputResolvedRow({
  outcome,
  redactedLength,
  supersededBy,
  resolvedAt,
}: TerminalInputResolvedRowProps) {
  const Glyph = RESOLVED_GLYPHS[outcome];
  return (
    <div
      className="flex flex-none items-center gap-2 border-line border-t bg-canvas px-3.5 py-3"
      data-outcome={outcome}
      data-testid={`terminal-input-resolved-${outcome}`}
    >
      <span
        className={cn(
          "grid size-6.5 flex-none place-items-center rounded-sm",
          RESOLVED_TONES[outcome]
        )}
      >
        <Glyph aria-hidden="true" className="size-3.5" />
      </span>
      <span className="text-muted text-transcript-meta">
        <b className="font-semibold text-fg">{terminalInputOutcomeCopy(outcome)}</b>
        {outcome === "answered" && redactedLength !== undefined
          ? ` · hidden input, ${redactedLength} characters`
          : null}
        {outcome === "rejected" ? " · the agent was told no input is coming" : null}
        {outcome === "superseded" && supersededBy
          ? ` · ${supersededBy} took control of the terminal`
          : null}
        {outcome === "expired" ? " · unanswered for 15 minutes" : null}
        {" · "}
        <Time iso={resolvedAt} />
      </span>
    </div>
  );
}
