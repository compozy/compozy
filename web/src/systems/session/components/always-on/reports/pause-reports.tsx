/**
 * S2 and the context and wake reports: why it stopped, what's waiting, and the one way forward.
 * Pure views; `unattended-pause-report.tsx` owns the resume they offer.
 */
import { Bell, Pause, Play } from "@compozy/ui/icons";
import { use, useState, type ReactNode } from "react";

import { Button, Receipt } from "@compozy/ui";

import { InlineCodeText } from "@/systems/agent/components/inline-code-text";

import { blockerPhrases, heldPhrase } from "../../../lib/session-pause";
import { SessionReportSendContext } from "../../../lib/session-report-send";
import type { ReportAgent } from "../../../lib/session-report-identity";
import { Notice } from "./report-entry";

/** The Resume control as it looks pending and failed; the caller owns the mutation. */
export function ResumeButton({
  label = "Resume",
  pending = false,
  failed = false,
  reason,
  disabled = false,
  onResume,
}: {
  label?: string;
  pending?: boolean;
  failed?: boolean;
  /** The daemon's reason for refusing, such as the active-agent cap. */
  reason?: string;
  disabled?: boolean;
  onResume?: () => void;
}) {
  return (
    <span className="flex flex-wrap items-center gap-2">
      <Button disabled={pending || disabled} onClick={onResume} size="sm" type="button">
        <Play aria-hidden="true" />
        {pending ? "Resuming…" : label}
      </Button>
      {failed ? (
        <span className="text-small-body text-destructive" role="alert">
          Couldn't resume. {reason ? <InlineCodeText text={reason} /> : "Try again."}
        </span>
      ) : null}
    </span>
  );
}

function AskAction({ prompt, children }: { prompt: string; children: ReactNode }) {
  const send = use(SessionReportSendContext);
  if (!send) return null;
  return (
    <Button onClick={() => send(prompt)} size="sm" type="button" variant="secondary">
      {children}
    </Button>
  );
}

/** The brake tripped: it stopped and says what's waiting. */
export function PauseNotice({
  agent,
  steps,
  held,
  when,
  action,
}: {
  agent: ReportAgent;
  /** The limit it reached; 0 when unknown. */
  steps: number;
  held: number;
  when: string;
  action?: ReactNode;
}) {
  const waiting = heldPhrase(held);
  return (
    <Notice
      actions={action}
      agent={agent}
      body={`I took ${steps || "many"} steps without hearing from you, so I stopped here. You can still message me; I'll answer and stay paused.`}
      detail={waiting ? <p className="text-body text-fg">{waiting} for you to resume.</p> : null}
      state="paused"
      title="Paused until you're back"
      when={when}
    />
  );
}

/** What the report settles into once it runs again. */
export function ResumedReceipt({ agent }: { agent: ReportAgent }) {
  return (
    <Receipt icon={<Play aria-hidden="true" />}>
      You resumed <b>{agent.label}</b>.
    </Receipt>
  );
}

/** A pause that's no longer the live one, or that another pause took over: a record, no action. */
export function PauseRecord({
  agent,
  steps,
  when,
}: {
  agent: ReportAgent;
  steps: number;
  when: string;
}) {
  return (
    <Receipt icon={<Pause aria-hidden="true" />}>
      <b>{agent.label}</b> paused at {when} after {steps || "many"} steps without you.
    </Receipt>
  );
}

/** A wake it was waiting on never reached it (rule 21). */
export function WakeLostReport({
  agent,
  detail,
  when,
}: {
  agent: ReportAgent;
  detail: string | undefined;
  when: string;
}) {
  const waitingFor = detail ? `I was waiting for ${detail.replace(/\.$/, "")}` : "I was waiting";
  return (
    <Notice
      actions={
        <AskAction prompt="Check whether what you were waiting for happened, and tell me what you find.">
          Check it now
        </AskAction>
      }
      agent={agent}
      body={`${waitingFor}, but that update never reached me. I may have missed the result.`}
      state="failed"
      title="A wake-up didn't arrive"
      when={when}
    />
  );
}

/** The provider refused the prompt: the conversation is full, and moves on once nothing's in the way. */
export function ContextFullReport({ agent, when }: { agent: ReportAgent; when: string }) {
  return (
    <Notice
      agent={agent}
      body="This conversation is full. I'll continue in a new one as soon as nothing is in the way. Until then, I can't take on anything else."
      state="failed"
      title="I'm out of room in this conversation"
      when={when}
    />
  );
}

/** Full, and the move to a new conversation is waiting on work still in progress. */
export function ContextBlockedReport({
  agent,
  blockers,
  when,
}: {
  agent: ReportAgent;
  blockers: readonly string[];
  when: string;
}) {
  const [open, setOpen] = useState(false);
  const phrases = blockerPhrases(blockers);
  const lead = phrases[0];
  return (
    <Notice
      actions={
        phrases.length > 1 ? (
          <Button
            aria-expanded={open}
            onClick={() => setOpen(value => !value)}
            size="sm"
            type="button"
            variant="secondary"
          >
            <Bell aria-hidden="true" />
            See what's in the way
          </Button>
        ) : null
      }
      agent={agent}
      body={
        lead
          ? `This conversation is full, and ${lead}. I'll start a new one as soon as that's done. Until then, I can't take on anything else.`
          : "This conversation is full. I'll start a new one as soon as other work finishes. Until then, I can't take on anything else."
      }
      detail={
        open ? (
          <ul aria-label="What's in the way" className="list-disc pl-5 text-body text-fg">
            {phrases.map(phrase => (
              <li key={phrase}>{phrase[0]?.toUpperCase() + phrase.slice(1)}</li>
            ))}
          </ul>
        ) : null
      }
      state="failed"
      title="I'm out of room in this conversation"
      when={when}
    />
  );
}

/** The first thing in the new conversation: why it started, before the link back. */
export function ContextRotatedNotice({ agent, when }: { agent: ReportAgent; when: string }) {
  return (
    <Notice
      agent={agent}
      body="This conversation filled up, so I'm continuing in a new one with a summary of the work so far."
      title="I started a new conversation"
      when={when}
    />
  );
}
