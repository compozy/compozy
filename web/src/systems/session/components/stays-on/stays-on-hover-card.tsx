import { Pause, Play } from "@compozy/ui/icons";
import { useQuery } from "@tanstack/react-query";

import {
  AGENT_AVATAR_STATE_LABEL,
  Button,
  Eyebrow,
  HoverCardContent,
  Pill,
  cn,
  type AgentAvatarState,
  type PillTone,
} from "@compozy/ui";

import { AgentMark } from "@/systems/agent/components/agent-mark";
import {
  AGENT_PAUSE_COPY,
  agentAvatarAppearance,
  agentInstanceAvatarState,
  agentInstanceCheckInsOptions,
  agentInstanceResumeBlocked,
  agentScopeLabel,
  type AgentInstanceCheckIn,
  type AgentInstancePayload,
} from "@/systems/agent";

import { formatRelativeClock } from "@/systems/agent";

const STATE_TONE: Partial<Record<AgentAvatarState, PillTone>> = {
  "needs-you": "brand",
  working: "success",
  failed: "destructive",
};

type CheckMark = "quiet" | "reported" | "failed";

const MARK: Record<CheckMark, string> = {
  quiet: "h-1.5 bg-viz-other",
  reported: "h-3 bg-viz-line",
  failed: "h-3 bg-destructive",
};

function checkMark({ result }: AgentInstanceCheckIn): CheckMark {
  if (result === "quiet") return "quiet";
  return result === "failed" ? "failed" : "reported";
}

export function CheckTicks({ checkIns }: { checkIns: readonly AgentInstanceCheckIn[] }) {
  const marks = checkIns.map(checkMark);
  const quiet = marks.filter(mark => mark === "quiet").length;
  return (
    <div
      aria-label={`Last ${marks.length} check-ins, ${quiet} with nothing to report`}
      className="flex h-3 items-center gap-0.5"
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

/** Reads only when a card opens; an instance with no check-ins yet shows nothing here. */
function CheckIns({ instance }: { instance: AgentInstancePayload }) {
  const { data } = useQuery(agentInstanceCheckInsOptions(instance.id));
  const checkIns = data?.check_ins ?? [];
  const last = formatRelativeClock(instance.last_wake_at);
  const next = instance.state === "paused" ? null : formatRelativeClock(instance.next_wake_at);
  const line = [last ? `Last check ${last}` : null, next ? `next ${next}` : null]
    .filter(Boolean)
    .join(", ");
  if (checkIns.length === 0 && !line) return null;
  return (
    <div className="flex flex-col gap-1.5">
      {checkIns.length > 0 ? (
        <>
          <Eyebrow className="text-subtle">Last {checkIns.length} check-ins</Eyebrow>
          <CheckTicks checkIns={checkIns} />
        </>
      ) : null}
      {line ? <span className="text-eyebrow text-subtle">{line}</span> : null}
    </div>
  );
}

function doingLine(instance: AgentInstancePayload) {
  if (instance.pause_reason) return AGENT_PAUSE_COPY[instance.pause_reason];
  return instance.now?.trim() || null;
}

/** What the agent is doing, one pointer away: the card behind every avatar in a row. */
export function StaysOnHoverCardContent({
  instance,
  onOpen,
  onTogglePause,
}: {
  instance: AgentInstancePayload;
  onOpen: () => void;
  onTogglePause: () => void;
}) {
  const state = agentInstanceAvatarState(instance);
  const paused = instance.state === "paused";
  const doing = doingLine(instance);
  return (
    <HoverCardContent align="start" className="flex w-80 flex-col gap-3.5 p-4" sideOffset={8}>
      <div className="flex items-center gap-3">
        <AgentMark
          appearance={agentAvatarAppearance(instance.resolved_appearance, instance.agent_name)}
          label={instance.label}
          provider={instance.provider}
          quietIdle
          size={48}
          state={state}
          surfaceClassName="ring-popover"
        />
        <div className="flex min-w-0 flex-1 flex-col gap-0.5">
          <span className="truncate text-heading font-medium text-fg-strong">{instance.label}</span>
          <span className="truncate text-small-body text-muted-foreground">
            {agentScopeLabel(instance.scope)}
          </span>
        </div>
        {state === "unknown" ? null : (
          <Pill size="sm" tone={STATE_TONE[state] ?? "neutral"}>
            {AGENT_AVATAR_STATE_LABEL[state]}
          </Pill>
        )}
      </div>
      {doing ? <p className="text-body text-fg">{doing}</p> : null}
      <CheckIns instance={instance} />
      <div className="flex gap-2">
        <Button className="flex-1" onClick={onOpen} size="sm" type="button" variant="secondary">
          Open conversation
        </Button>
        <Button
          aria-label={paused ? "Resume" : "Pause"}
          disabled={paused && agentInstanceResumeBlocked(instance)}
          onClick={onTogglePause}
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
