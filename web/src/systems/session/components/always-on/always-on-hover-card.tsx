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
  useMarkAgentInstanceSeen,
  agentNextWakeLine,
  agentPauseCopy,
  agentAuthorityCopy,
  agentAvatarAppearance,
  agentInstanceAvatarState,
  agentInstanceCheckInsOptions,
  agentInstanceResumeBlocked,
  agentScopeLabel,
  type AgentInstanceCheckIn,
  type AgentInstancePayload,
} from "@/systems/agent";

import { formatRelativeClock } from "@/systems/agent";

import { heldPhrase } from "../../lib/session-pause";

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
      aria-label={`${marks.length === 1 ? "Last heartbeat" : `Last ${marks.length} heartbeats`}, ${quiet} with nothing to report`}
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

/** Reads only when a card opens; an instance with no heartbeats yet shows nothing here. */
function CheckIns({ instance }: { instance: AgentInstancePayload }) {
  const { data } = useQuery(agentInstanceCheckInsOptions(instance));
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
          <Eyebrow className="text-subtle">
            {checkIns.length === 1 ? "Last heartbeat" : `Last ${checkIns.length} heartbeats`}
          </Eyebrow>
          <CheckTicks checkIns={checkIns} />
        </>
      ) : null}
      {line ? <span className="text-eyebrow text-subtle">{line}</span> : null}
    </div>
  );
}

/** S7: what wakes it next, so a sleeping agent says what it's waiting for. */
function NextWake({ instance }: { instance: AgentInstancePayload }) {
  // The next heartbeat already reads in the heartbeats line above.
  const line = instance.next_wake?.kind === "check" ? null : agentNextWakeLine(instance);
  if (!line) return null;
  return (
    <div className="flex flex-col gap-0.5">
      <Eyebrow className="text-subtle">Wakes up next</Eyebrow>
      <span className="text-small-body text-fg">{line}</span>
    </div>
  );
}

function doingLine(instance: AgentInstancePayload) {
  return agentPauseCopy(instance) || instance.now.trim() || null;
}

/** What the agent is doing, one pointer away: the card behind every avatar in a row. */
export function AlwaysOnHoverCardContent({
  instance,
  onOpen,
  onTogglePause,
}: {
  instance: AgentInstancePayload;
  onOpen: () => void;
  onTogglePause: () => void;
}) {
  const markSeen = useMarkAgentInstanceSeen();
  const state = agentInstanceAvatarState(instance);
  const paused = instance.state === "paused";
  const doing = doingLine(instance);
  const held = heldPhrase(instance.held_wakes ?? 0);
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
      <NextWake instance={instance} />
      {paused && held ? (
        <p className="text-small-body text-muted-foreground">{held} for you to resume</p>
      ) : null}
      {agentAuthorityCopy(instance) ? (
        <p className="text-small-body text-warning">{agentAuthorityCopy(instance)}</p>
      ) : null}
      {instance.attention_dismissible ? (
        <Button
          disabled={markSeen.isPending}
          onClick={() => markSeen.mutate(instance)}
          size="sm"
          type="button"
          variant="quiet"
        >
          {markSeen.isPending ? "Marking seen…" : "Mark as seen"}
        </Button>
      ) : null}
      {markSeen.error ? (
        <p role="alert" className="text-small-body text-destructive">
          {markSeen.error.message}
        </p>
      ) : null}
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
