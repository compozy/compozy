/** S2, S3 and the context reports: why it stopped, what's waiting, and the one way forward. */
import { Bell, GitPullRequest, Play, Users } from "@compozy/ui/icons";
import { useState } from "react";

import { Button, Eyebrow, Item, Receipt, Separator } from "@compozy/ui";

import { AgentAvatar } from "../stays-on-agents-shared/agent-avatar";
import { TRIAGE } from "../stays-on-agents-shared/fixtures";
import { AgentTurn, UserTurn } from "../stays-on-agents-shared/transcript";
import { Notice } from "../stays-on-reports/report-entry";
import { RotatedMarker } from "../stays-on-reports/session-views";

const WAITING = [
  { icon: GitPullRequest, label: "CI finished on #1166" },
  { icon: Users, label: "Code Engineer finished the web slice" },
] as const;

function Waiting() {
  return (
    <div className="flex flex-col gap-1">
      <Eyebrow className="text-subtle">Waiting for you to resume</Eyebrow>
      <ul>
        {WAITING.map(item => (
          <li key={item.label}>
            <Item className="gap-2.5 px-0 py-1">
              <item.icon aria-hidden="true" className="size-4 shrink-0 text-subtle" />
              <span className="text-body text-fg">{item.label}</span>
            </Item>
          </li>
        ))}
      </ul>
    </div>
  );
}

function ResumeButton({ onResume }: { onResume?: () => void }) {
  return (
    <Button onClick={onResume} size="sm" type="button">
      <Play aria-hidden="true" />
      Resume
    </Button>
  );
}

export function PauseReport({ onResume }: { onResume?: () => void }) {
  return (
    <Notice
      actions={<ResumeButton onResume={onResume} />}
      agent={TRIAGE}
      body="I took 300 steps without hearing from you, so I stopped here. You can still message me; I'll answer and stay paused."
      detail={<Waiting />}
      state="paused"
      title="Paused until you're back"
      when="04:12"
    />
  );
}

/** Resume in place: the report settles into a record and the waiting work runs. */
export function ResumablePause() {
  const [resumed, setResumed] = useState(false);
  return resumed ? <ResumedReceipt /> : <PauseReport onResume={() => setResumed(true)} />;
}

export function ResumedReceipt() {
  return (
    <>
      <Receipt icon={<Play aria-hidden="true" />}>
        You resumed <b>Triage Lead</b>. Delivering 2 things that were waiting.
      </Receipt>
      <AgentTurn>CI is green on #1166, and the web slice is ready for review.</AgentTurn>
    </>
  );
}

export function AwayPause() {
  return (
    <section aria-label="While you were away" className="flex flex-col gap-4">
      <Separator label="While you were away" />
      <div className="flex flex-col gap-4 rounded-lg bg-card p-5 shadow-card">
        <div className="flex items-center gap-3.5">
          <AgentAvatar
            appearance={TRIAGE.appearance}
            label="Triage Lead"
            quietIdle
            size={48}
            state="paused"
          />
          <div className="flex min-w-0 flex-1 flex-col gap-0.5">
            <h3 className="text-detail-h1 font-medium text-fg-strong">
              Five hours of work, then it paused.
            </h3>
            <p className="text-body text-muted-foreground">
              Triage Lead reached its limit at 04:12. 2 things are waiting.
            </p>
          </div>
          <ResumeButton />
        </div>
      </div>
    </section>
  );
}

export function PromptWhilePaused() {
  return (
    <>
      <PauseReport />
      <UserTurn>What's left on the release?</UserTurn>
      <AgentTurn>Two things: the CI result on #1166 and the review of the web slice.</AgentTurn>
    </>
  );
}

export function WakeLost() {
  return (
    <Notice
      actions={
        <Button size="sm" type="button" variant="secondary">
          Check it now
        </Button>
      }
      agent={TRIAGE}
      body="I was waiting for CI on #1166, but that update never reached me. I may have missed the result."
      state="failed"
      title="A wake-up didn't arrive"
      when="03:02"
    />
  );
}

export function ContextFullBlocked() {
  return (
    <Notice
      actions={
        <Button size="sm" type="button" variant="secondary">
          <Bell aria-hidden="true" />
          See what's in the way
        </Button>
      }
      agent={TRIAGE}
      body="This conversation is full, and Code Engineer is still running. I'll start a new one as soon as it finishes. Until then, I can't take on anything else."
      state="failed"
      title="I'm out of room in this conversation"
      when="05:48"
    />
  );
}

export function ContextFullRotated() {
  return (
    <>
      <Notice
        agent={TRIAGE}
        body="This conversation filled up, so I'm continuing in a new one with a summary of the work so far."
        title="I started a new conversation"
        when="05:48"
      />
      <RotatedMarker />
    </>
  );
}
