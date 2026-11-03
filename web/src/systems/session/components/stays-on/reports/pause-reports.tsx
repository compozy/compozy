/**
 * S2 and the context and wake reports: why it stopped, what's waiting, and the one way forward.
 * Resume happens in place; the report settles into a record once the agent runs again.
 */
import { Bell, Play } from "@compozy/ui/icons";
import { use, useState, type ReactNode } from "react";

import { Button, Receipt } from "@compozy/ui";

import {
  agentInstancePausedUnattended as pausedUnattended,
  agentUnattendedSteps,
  useResumeAgentInstances,
  type AgentInstancePayload,
} from "@/systems/agent";

import { blockerPhrases, deliveredPhrase, heldPhrase } from "../../../lib/session-pause";
import { SessionReportSendContext } from "../../../lib/session-report-send";
import type { ReportAgent } from "../../../lib/session-report-identity";
import { Notice } from "./report-entry";

/** Resume with its pending and failed states, for every place a paused report sits. */
export function ResumeAction({
  instances,
  label = "Resume",
  onResumed,
}: {
  instances: readonly AgentInstancePayload[];
  label?: string;
  onResumed?: () => void;
}) {
  const resume = useResumeAgentInstances();
  return (
    <span className="flex flex-wrap items-center gap-2">
      <Button
        disabled={resume.isPending || instances.length === 0}
        onClick={() => resume.mutate(instances, { onSuccess: onResumed })}
        size="sm"
        type="button"
      >
        <Play aria-hidden="true" />
        {resume.isPending ? "Resuming…" : label}
      </Button>
      {resume.isError ? (
        <span className="text-small-body text-destructive" role="alert">
          Couldn't resume. Try again.
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

/** The brake tripped: it stopped, says what's waiting, and resumes in place. */
export function UnattendedPauseReport({
  agent,
  instance,
  when,
}: {
  agent: ReportAgent;
  instance: AgentInstancePayload | undefined;
  when: string;
}) {
  // What was waiting when you resumed here, so the record can say what it delivered.
  const [delivered, setDelivered] = useState<number | null>(null);
  if (instance && !pausedUnattended(instance)) {
    const handed = deliveredPhrase(delivered ?? 0);
    return (
      <Receipt icon={<Play aria-hidden="true" />}>
        You resumed <b>{agent.label}</b>.{handed ? ` ${handed}` : null}
      </Receipt>
    );
  }
  const steps = instance ? agentUnattendedSteps(instance) : 0;
  const held = heldPhrase(instance?.held_wakes ?? 0);
  return (
    <Notice
      actions={
        instance ? (
          <ResumeAction
            instances={[instance]}
            onResumed={() => setDelivered(instance.held_wakes ?? 0)}
          />
        ) : null
      }
      agent={agent}
      body={`I took ${steps || "many"} steps without hearing from you, so I stopped here. You can still message me; I'll answer and stay paused.`}
      detail={held ? <p className="text-body text-fg">{held} for you to resume.</p> : null}
      state="paused"
      title="Paused until you're back"
      when={when}
    />
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
