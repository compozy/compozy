/** Where a waiting script shows outside Heartbeat: the agents list row and the agent's profile. */
import { ArrowRight } from "@compozy/ui/icons";

import { Alert, AlertDescription, AlertTitle, Button, Eyebrow, Pill } from "@compozy/ui";

import { AgentMark } from "@/systems/agent/components/agent-mark";
import { scriptApprovalNoticesFixture } from "@/systems/agent/mocks/script-approval-fixtures";

/** The pill an agent shows wherever it's listed while one of its scripts waits. */
export function ScriptWaitingPill() {
  return (
    <Pill form="plain" tone="warning">
      <Pill.Dot />1 script to approve
    </Pill>
  );
}

function AgentListRow({
  label,
  seed,
  paused,
  waiting,
}: {
  label: string;
  seed: string;
  paused: boolean;
  waiting: boolean;
}) {
  return (
    <div className="flex min-h-12 items-center gap-3 px-3 py-2">
      <AgentMark
        appearance={{ avatar: "generated", seed }}
        label={label}
        quietIdle
        size={28}
        state={paused ? "paused" : waiting ? "needs-you" : "idle"}
      />
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-small-body font-medium text-fg">{label}</span>
        <span className="truncate text-eyebrow text-subtle">
          {paused ? "Paused · compozy" : "Resting · compozy"}
        </span>
      </span>
      {waiting ? <ScriptWaitingPill /> : null}
    </div>
  );
}

/** The profile's alert, beside the "Stopped" alert it already shows. */
export function ProfileAlert() {
  const [first] = scriptApprovalNoticesFixture;
  return (
    <Alert variant="warning">
      <AlertTitle>{first!.script.name} is waiting for your approval</AlertTitle>
      <AlertDescription>
        <p>Heartbeat won't run it until you approve it.</p>
        <Button className="mt-2" size="sm" type="button" variant="secondary">
          Open Heartbeat
          <ArrowRight aria-hidden="true" data-icon="inline-end" />
        </Button>
      </AlertDescription>
    </Alert>
  );
}

export function ScriptApprovalBadges() {
  return (
    <div className="grid max-w-5xl grid-cols-2 items-start gap-6 p-6">
      <div className="flex flex-col gap-2">
        <Eyebrow className="text-subtle">Agents list</Eyebrow>
        <div className="flex flex-col divide-y divide-line-soft rounded-lg bg-card shadow-card">
          {scriptApprovalNoticesFixture.map(notice => (
            <AgentListRow
              key={notice.id}
              label={notice.agentLabel}
              paused={notice.paused}
              seed={notice.agentName}
              waiting
            />
          ))}
          <AgentListRow label="Triage Lead" paused={false} seed="triage" waiting={false} />
        </div>
      </div>
      <div className="flex flex-col gap-2">
        <Eyebrow className="text-subtle">Agent profile</Eyebrow>
        <ProfileAlert />
      </div>
    </div>
  );
}
