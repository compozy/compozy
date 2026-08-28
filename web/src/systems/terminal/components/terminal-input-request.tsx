"use client";

import { Check, Clock, KeyRound, Keyboard, MessageCircleQuestionMark, X } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useEffect, useId, useState, type ReactNode } from "react";

import { Button, cn, Input, MonoId, Time } from "@compozy/ui";

import { terminalInputOutcomeCopy } from "../lib/terminal-copy";
import { terminalInputExpiry } from "../lib/terminal-input-expiry";
import { terminalInputRequestTitle } from "../lib/terminal-input-identity";
import { terminalRedactedInputCopy } from "../lib/terminal-redacted-marker";
import type {
  TerminalInputOutcome,
  TerminalInputRequest,
  TerminalResolvedInputRequest,
} from "../types";

export interface TerminalInputRequestCardProps {
  request: TerminalInputRequest;
  /** False for a watcher or all-profiles read: the write row is absent. */
  canAnswerDirectly: boolean;
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

function RequestGlyph({ redacted }: { redacted: boolean }) {
  const Glyph = redacted ? KeyRound : MessageCircleQuestionMark;
  return (
    <span className="grid size-6.5 flex-none place-items-center rounded-sm bg-warning-tint text-warning">
      <Glyph aria-hidden="true" className="size-3.5" />
    </span>
  );
}

function RequestPin({
  request,
  showOrigin,
  terminalTitle,
  expiryLabel,
  expired,
  titleId,
  children,
}: {
  request: TerminalInputRequest;
  showOrigin: boolean;
  terminalTitle?: string;
  expiryLabel: string;
  expired: boolean;
  titleId: string;
  children?: ReactNode;
}) {
  return (
    <div
      className="flex flex-col gap-2.5 border-line border-t bg-canvas px-3.5 py-3"
      data-redacted={request.redacted ? "true" : undefined}
      data-testid={`terminal-input-request-${request.id}`}
    >
      <div className="flex w-full min-w-0 items-center gap-2">
        <RequestGlyph redacted={request.redacted} />
        <span
          className="min-w-0 truncate text-small-body font-semibold text-fg-strong"
          id={titleId}
        >
          {terminalInputRequestTitle(request)}
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
            expired ? "text-muted" : "text-faint"
          )}
          data-testid={`terminal-input-request-expiry-${request.id}`}
        >
          {expiryLabel}
        </span>
      </div>
      <p className="text-small-body text-muted">{request.reason}</p>
      <div className="rounded-xs bg-chat-fill-code px-2.25 py-1.5 font-mono text-badge leading-normal break-all whitespace-pre-wrap text-fg">
        {request.prompt_excerpt}
      </div>
      {children}
    </div>
  );
}

/**
 * The agent's question, pinned to the terminal it belongs to.
 *
 * A redacted answer never renders, is never stored, and never reaches the
 * agent: the field masks it, and the only thing that survives anywhere is the
 * fact that hidden input of some length happened.
 *
 * The field stays uncontrolled: a controlled one would put the secret in React
 * state and in the element's serialized `value` attribute, where a snapshot or
 * an error report could carry it off; here it exists only in the live DOM node
 * until the form is submitted and reset.
 *
 * A watcher, all-profiles read, or expired pin sees the question and no write
 * row — take control lives on the header, not on this pin, and a lapsed
 * request cannot send.
 */
export function TerminalInputRequestCard({
  request,
  canAnswerDirectly,
  showOrigin = false,
  terminalTitle,
  onAnswer,
  onReject,
  now,
}: TerminalInputRequestCardProps) {
  const titleId = useId();
  const [liveNow, setLiveNow] = useState(() => now ?? Date.now());
  useEffect(() => {
    if (now !== undefined) return undefined;
    const timer = window.setInterval(() => setLiveNow(Date.now()), EXPIRY_REFRESH_MS);
    return () => window.clearInterval(timer);
  }, [now]);
  const expiry = terminalInputExpiry(request.requested_at, now ?? liveNow);
  const canWrite = canAnswerDirectly && !expiry.expired;
  return (
    <RequestPin
      expired={expiry.expired}
      expiryLabel={expiry.label}
      request={request}
      showOrigin={showOrigin}
      terminalTitle={terminalTitle}
      titleId={titleId}
    >
      {canWrite ? (
        <form
          className="flex items-center gap-2"
          onSubmit={event => {
            event.preventDefault();
            const form = event.currentTarget;
            const answer = new FormData(form).get(INPUT_FIELD_NAME);
            onAnswer(typeof answer === "string" ? answer : "");
            form.reset();
          }}
        >
          <div className="min-w-0 flex-1">
            <Input
              aria-describedby={titleId}
              aria-label={
                request.redacted
                  ? "Hidden input — never shown, stored, or sent to the agent"
                  : "Your answer"
              }
              autoComplete="off"
              className={cn(request.redacted && "font-mono tracking-redacted")}
              data-testid={`terminal-input-request-field-${request.id}`}
              name={INPUT_FIELD_NAME}
              type={request.redacted ? "password" : "text"}
            />
          </div>
          <Button
            data-testid={`terminal-input-request-send-${request.id}`}
            size="sm"
            type="submit"
            variant="neutral"
          >
            Send
          </Button>
          <Button
            data-testid={`terminal-input-request-decline-${request.id}`}
            onClick={onReject}
            size="sm"
            type="button"
            variant="ghost"
          >
            Decline
          </Button>
        </form>
      ) : null}
    </RequestPin>
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
  request: TerminalResolvedInputRequest;
  /** Who took over, on `superseded`, when the surface knows a name. */
  supersededBy?: string;
}

/**
 * A resolved question, collapsed to one quiet line.
 *
 * An answered redacted prompt states its length and nothing else — that marker
 * is the whole record, in the stream, the journal, and the replay alike.
 */
export function TerminalInputResolvedRow({ request, supersededBy }: TerminalInputResolvedRowProps) {
  const Glyph = RESOLVED_GLYPHS[request.outcome];
  return (
    <div
      className="flex flex-none items-center gap-2 border-line border-t bg-canvas px-3.5 py-3"
      data-outcome={request.outcome}
      data-testid={`terminal-input-resolved-${request.outcome}`}
    >
      <span
        className={cn(
          "grid size-6.5 flex-none place-items-center rounded-sm",
          RESOLVED_TONES[request.outcome]
        )}
      >
        <Glyph aria-hidden="true" className="size-3.5" />
      </span>
      <span className="text-muted text-transcript-meta">
        <b className="font-semibold text-fg">
          {terminalInputOutcomeCopy(request.outcome, request.resolved_by)}
        </b>
        {request.outcome === "answered" && request.redacted
          ? ` · ${terminalRedactedInputCopy(request.length)}`
          : null}
        {request.outcome === "rejected" ? " · the agent was told no input is coming" : null}
        {request.outcome === "superseded"
          ? ` · ${supersededBy ?? request.resolved_by.id} took control of the terminal`
          : null}
        {request.outcome === "expired" ? " · unanswered for 15 minutes" : null}
        {" · "}
        <Time iso={request.resolved_at} />
      </span>
    </div>
  );
}
