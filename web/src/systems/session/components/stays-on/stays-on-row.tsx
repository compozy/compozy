import { Archive, Bot, HeartPulse, Pause, Play } from "@compozy/ui/icons";
import type { ReactNode } from "react";

import {
  AGENT_AVATAR_STATE_LABEL,
  ContextMenu,
  ContextMenuContent,
  ContextMenuGroup,
  ContextMenuItem,
  ContextMenuLabel,
  ContextMenuSeparator,
  ContextMenuTrigger,
  HoverCard,
  HoverCardTrigger,
  cn,
} from "@compozy/ui";

import { AgentMark } from "@/systems/agent/components/agent-mark";
import {
  agentPauseCopy,
  agentAvatarAppearance,
  agentInstanceAvatarState,
  agentInstanceResumeBlocked,
  agentInstancesByUrgency,
  agentScopeLabel,
  type AgentInstancePayload,
} from "@/systems/agent";

import { useStaysOnRowActions, type StaysOnRowActions } from "../../hooks/use-stays-on-row";
import type { SessionListTarget } from "../../types";
import { StaysOnCrewPopover } from "./stays-on-crew-popover";
import { StaysOnHoverCardContent } from "./stays-on-hover-card";

const AVATAR_PX = 30;

const SEAT =
  "grid size-9 shrink-0 cursor-pointer place-items-center rounded-full outline-none transition-[background-color,translate] duration-base hover:-translate-y-0.5 hover:bg-hover focus-visible:shadow-focus-ring motion-reduce:transition-none motion-reduce:hover:translate-y-0";

function spoken(instance: AgentInstancePayload) {
  const state = agentInstanceAvatarState(instance);
  return [
    instance.label,
    agentScopeLabel(instance.scope),
    ...(state === "unknown" ? [] : [AGENT_AVATAR_STATE_LABEL[state]]),
  ].join(" · ");
}

function SeatMenu({
  instance,
  actions,
}: {
  instance: AgentInstancePayload;
  actions: StaysOnRowActions;
}) {
  const paused = instance.state === "paused";
  const reason = agentPauseCopy(instance);
  return (
    <ContextMenuContent className="w-64">
      <ContextMenuGroup>
        <ContextMenuLabel>
          {instance.label} · {agentScopeLabel(instance.scope)}
        </ContextMenuLabel>
        {reason ? <p className="px-2 pb-1.5 text-eyebrow text-subtle">{reason}</p> : null}
      </ContextMenuGroup>
      <ContextMenuSeparator />
      <ContextMenuItem
        disabled={paused && agentInstanceResumeBlocked(instance)}
        onClick={() => actions.togglePause(instance)}
      >
        {paused ? <Play aria-hidden="true" /> : <Pause aria-hidden="true" />}
        {paused ? "Resume" : "Pause"}
      </ContextMenuItem>
      <ContextMenuItem onClick={() => void actions.openAgent(instance)}>
        <Bot aria-hidden="true" />
        Open agent
      </ContextMenuItem>
      <ContextMenuItem onClick={() => void actions.openCheckIns(instance)}>
        <HeartPulse aria-hidden="true" />
        Check-ins
      </ContextMenuItem>
      <ContextMenuSeparator />
      <ContextMenuItem onClick={() => actions.archive(instance)}>
        <Archive aria-hidden="true" />
        Archive
      </ContextMenuItem>
    </ContextMenuContent>
  );
}

function Seat({
  instance,
  surface,
  current,
  actions,
}: {
  instance: AgentInstancePayload;
  surface: string;
  current: boolean;
  actions: StaysOnRowActions;
}) {
  return (
    <ContextMenu>
      <HoverCard>
        <ContextMenuTrigger
          render={
            <HoverCardTrigger
              render={
                <button
                  aria-current={current ? "page" : undefined}
                  aria-label={spoken(instance)}
                  className={cn(SEAT, current && "bg-selected")}
                  data-testid={`stays-on-seat-${instance.id}`}
                  onClick={() => void actions.open(instance)}
                  type="button"
                />
              }
            />
          }
        >
          <AgentMark
            appearance={agentAvatarAppearance(instance.resolved_appearance, instance.agent_name)}
            label={instance.label}
            provider={instance.provider}
            quietIdle
            scope={agentScopeLabel(instance.scope)}
            size={AVATAR_PX}
            state={agentInstanceAvatarState(instance)}
            surfaceClassName={surface}
          />
        </ContextMenuTrigger>
        <StaysOnHoverCardContent
          instance={instance}
          onOpen={() => void actions.open(instance)}
          onTogglePause={() => actions.togglePause(instance)}
        />
      </HoverCard>
      <SeatMenu actions={actions} instance={instance} />
    </ContextMenu>
  );
}

export interface StaysOnRowProps {
  instances: readonly AgentInstancePayload[];
  onOpenSession: (session: SessionListTarget) => void;
  currentSessionId?: string | null;
  /** Agents seated before the rest move into `+N`. */
  limit?: number;
  label?: ReactNode;
  /** The surface the avatars' badges cut into. */
  surface?: string;
  /** The scope's name, for the crew popover's title. */
  scopeName?: string;
  className?: string;
}

/**
 * S10 and S15: the agents who live in a scope, large enough to read a face, most urgent first, so
 * anyone who needs you leads the row. Each opens a card on hover or focus; `+N` opens the crew.
 */
export function StaysOnRow(props: StaysOnRowProps) {
  // No instances means no row, and nothing here reads the router or a mutation.
  return props.instances.length === 0 ? null : <StaysOnSeats {...props} />;
}

function StaysOnSeats({
  instances,
  onOpenSession,
  currentSessionId,
  limit = 6,
  label,
  surface = "ring-sunken",
  scopeName = "this project",
  className,
}: StaysOnRowProps) {
  const actions = useStaysOnRowActions(onOpenSession);
  const ordered = agentInstancesByUrgency(instances);
  const shown = ordered.slice(0, limit);
  const hidden = ordered.length - shown.length;
  return (
    <div
      aria-label="Stays-on agents"
      className={cn("flex min-w-0 items-center gap-1", className)}
      data-testid="stays-on-row"
      role="toolbar"
    >
      {label}
      {shown.map(instance => (
        <Seat
          actions={actions}
          current={instance.main_session_id === currentSessionId}
          instance={instance}
          key={instance.id}
          surface={surface}
        />
      ))}
      {hidden > 0 ? (
        <StaysOnCrewPopover
          hidden={hidden}
          instances={instances}
          onOpen={instance => void actions.open(instance)}
          scopeName={scopeName}
        />
      ) : null}
    </div>
  );
}
