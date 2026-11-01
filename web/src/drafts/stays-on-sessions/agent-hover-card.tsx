/** What the agent is doing, one pointer away: the card behind every avatar in a row. */
import { Pause, Play } from "lucide-react";

import { Button, Eyebrow, HoverCardContent, Pill, type PillTone, cn } from "@compozy/ui";

import { AgentAvatar } from "../stays-on-agents-shared/agent-avatar";
import { AGENT_STATE_LABEL, type AgentState } from "../stays-on-agents-shared/agent-state";
import { SCOPE_LABEL, type DraftInstance } from "../stays-on-agents-shared/fixtures";
import { checksOf, doingOf, type CheckMark } from "../stays-on-agents-shared/sessions-fixtures";
import { openMainSession } from "./open-main";

const STATE_TONE: Partial<Record<AgentState, PillTone>> = {
  "needs-you": "accent",
  working: "success",
  failed: "danger",
};

const MARK: Record<CheckMark, string> = {
  quiet: "h-1.5 bg-viz-other",
  reported: "h-3 bg-viz-line",
  failed: "h-3 bg-danger",
};

export function CheckTicks({
  marks,
  className,
}: {
  marks: readonly CheckMark[];
  className?: string;
}) {
  const quiet = marks.filter(mark => mark === "quiet").length;
  return (
    <div
      aria-label={`Last ${marks.length} check-ins, ${quiet} with nothing to report`}
      className={cn("flex h-3 items-center gap-0.5", className)}
      role="img"
    >
      {marks.map((mark, index) => (
        <span
          aria-hidden="true"
          className={cn("min-w-0 flex-1 rounded-xs", MARK[mark])}
          key={index}
        />
      ))}
    </div>
  );
}

function checkLine(instance: DraftInstance) {
  if (instance.state === "paused") return `Last check ${instance.lastCheck ?? "a while ago"}`;
  const parts = [
    instance.lastCheck ? `Last check ${instance.lastCheck}` : null,
    instance.nextCheck ? `next ${instance.nextCheck}` : null,
  ];
  return parts.filter(Boolean).join(", ");
}

export function AgentHoverCardContent({ instance }: { instance: DraftInstance }) {
  const paused = instance.state === "paused";
  const line = checkLine(instance);
  return (
    <HoverCardContent align="start" className="flex w-80 flex-col gap-3.5 p-4" sideOffset={8}>
      <div className="flex items-center gap-3">
        <AgentAvatar
          appearance={instance.agent.appearance}
          label={instance.agent.label}
          provider={instance.agent.provider}
          quietIdle
          size={48}
          state={instance.state}
          surfaceClassName="ring-popover"
        />
        <div className="flex min-w-0 flex-1 flex-col gap-0.5">
          <span className="truncate text-heading font-medium text-fg-strong">
            {instance.agent.label}
          </span>
          <span className="truncate text-small-body text-muted">{SCOPE_LABEL(instance.scope)}</span>
        </div>
        {instance.state === "unknown" ? null : (
          <Pill size="sm" tone={STATE_TONE[instance.state] ?? "neutral"}>
            {AGENT_STATE_LABEL[instance.state]}
          </Pill>
        )}
      </div>
      <p className="text-body text-fg">{doingOf(instance)}</p>
      <div className="flex flex-col gap-1.5">
        <Eyebrow className="text-subtle">Last 12 check-ins</Eyebrow>
        <CheckTicks marks={checksOf(instance)} />
        {line ? <span className="text-eyebrow text-subtle">{line}</span> : null}
      </div>
      <div className="flex gap-2">
        <Button
          className="flex-1"
          onClick={() => openMainSession(instance)}
          size="sm"
          type="button"
          variant="secondary"
        >
          Open conversation
        </Button>
        <Button
          aria-label={paused ? "Resume" : "Pause"}
          size="icon-sm"
          type="button"
          variant="quiet"
        >
          {paused ? <Play aria-hidden="true" /> : <Pause aria-hidden="true" />}
        </Button>
      </div>
    </HoverCardContent>
  );
}
