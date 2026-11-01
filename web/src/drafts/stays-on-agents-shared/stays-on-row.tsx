/**
 * S10 and S15: the agents who live in a project. They sit on their own, large enough to read a
 * face, most urgent first, so anyone who needs you leads the row and shows it on the avatar. Each
 * opens a card on hover or focus, and `+N` opens the whole crew.
 */
import { Archive, Bot, HeartPulse, Pause, Play } from "lucide-react";
import type { ReactNode } from "react";

import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  HoverCard,
  HoverCardTrigger,
  cn,
} from "@compozy/ui";

import { AgentHoverCardContent } from "../stays-on-sessions/agent-hover-card";
import { CrewPopover } from "../stays-on-sessions/crew-popover";
import { openMainSession } from "../stays-on-sessions/open-main";
import { AgentAvatar } from "./agent-avatar";
import { AGENT_STATE_LABEL } from "./agent-state";
import { PAUSE_COPY, SCOPE_LABEL, type DraftInstance } from "./fixtures";
import { byUrgency } from "./sessions-fixtures";

const AVATAR_PX = 30;

function spoken(instance: DraftInstance) {
  return [
    instance.agent.label,
    SCOPE_LABEL(instance.scope),
    AGENT_STATE_LABEL[instance.state],
  ].join(" · ");
}

function Mark({
  instance,
  surface,
  size = AVATAR_PX,
}: {
  instance: DraftInstance;
  surface: string;
  size?: number;
}) {
  return (
    <AgentAvatar
      appearance={instance.agent.appearance}
      label={instance.agent.label}
      provider={instance.agent.provider}
      quietIdle
      scope={SCOPE_LABEL(instance.scope)}
      size={size}
      state={instance.state}
      surfaceClassName={surface}
    />
  );
}

const SEAT =
  "grid size-9 shrink-0 cursor-pointer place-items-center rounded-full outline-none transition-[background-color,translate] duration-base hover:-translate-y-0.5 hover:bg-hover focus-visible:shadow-focus-ring motion-reduce:transition-none motion-reduce:hover:translate-y-0";

function ContextItems({ instance }: { instance: DraftInstance }) {
  const paused = instance.state === "paused";
  return (
    <ContextMenuContent>
      <ContextMenuItem>
        {paused ? <Play aria-hidden="true" /> : <Pause aria-hidden="true" />}
        {paused ? "Resume" : "Pause"}
      </ContextMenuItem>
      <ContextMenuItem>
        <Bot aria-hidden="true" />
        Open agent
      </ContextMenuItem>
      <ContextMenuItem>
        <HeartPulse aria-hidden="true" />
        Check-ins
      </ContextMenuItem>
      <ContextMenuSeparator />
      <ContextMenuItem>
        <Archive aria-hidden="true" />
        Archive
      </ContextMenuItem>
    </ContextMenuContent>
  );
}

/** The same verbs as the context menu, held open for review. */
function OpenMenu({ instance, surface }: { instance: DraftInstance; surface: string }) {
  const paused = instance.state === "paused";
  const blocked =
    instance.pauseReason === "definition_missing" ||
    instance.pauseReason === "definition_not_stays_on";
  return (
    <DropdownMenu defaultOpen>
      <DropdownMenuTrigger aria-label={spoken(instance)} className={SEAT}>
        <Mark instance={instance} surface={surface} />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-64">
        <DropdownMenuGroup>
          <DropdownMenuLabel>
            {instance.agent.label} · {SCOPE_LABEL(instance.scope)}
          </DropdownMenuLabel>
          {instance.pauseReason ? (
            <p className="px-2 pb-1.5 text-eyebrow text-subtle">
              {PAUSE_COPY[instance.pauseReason]}
            </p>
          ) : null}
        </DropdownMenuGroup>
        <DropdownMenuSeparator />
        <DropdownMenuItem disabled={blocked}>
          {paused ? <Play aria-hidden="true" /> : <Pause aria-hidden="true" />}
          {paused ? "Resume" : "Pause"}
        </DropdownMenuItem>
        <DropdownMenuItem>
          <Bot aria-hidden="true" />
          Open agent
        </DropdownMenuItem>
        <DropdownMenuItem>
          <HeartPulse aria-hidden="true" />
          Check-ins
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem>
          <Archive aria-hidden="true" />
          Archive
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function Seat({
  instance,
  surface,
  cardOpen,
  focused,
  current,
}: {
  instance: DraftInstance;
  surface: string;
  cardOpen: boolean;
  focused: boolean;
  current: boolean;
}) {
  return (
    <ContextMenu>
      <HoverCard open={cardOpen || undefined}>
        <ContextMenuTrigger
          render={
            <HoverCardTrigger
              render={
                <button
                  aria-current={current ? "page" : undefined}
                  aria-label={spoken(instance)}
                  className={cn(SEAT, focused && "shadow-focus-ring", current && "bg-selected")}
                  onClick={() => openMainSession(instance)}
                  type="button"
                />
              }
            />
          }
        >
          <Mark instance={instance} surface={surface} />
        </ContextMenuTrigger>
        <AgentHoverCardContent instance={instance} />
      </HoverCard>
      <ContextItems instance={instance} />
    </ContextMenu>
  );
}

export interface StaysOnRowProps {
  instances: readonly DraftInstance[];
  /** Agents seated before the rest move into `+N`. */
  limit?: number;
  label?: ReactNode;
  surface?: string;
  /** The scope's name, for the crew popover's title. */
  scopeName?: string;
  openMenuFor?: string;
  focusedId?: string;
  /** Holds one agent's hover card open for review. */
  cardFor?: string;
  /** Holds the crew popover open for review. */
  crewOpen?: boolean;
  /** The instance whose main session is open in this window. */
  currentId?: string;
  className?: string;
}

export function StaysOnRow({
  instances,
  limit = 6,
  label,
  surface = "ring-sunken",
  scopeName = "this project",
  openMenuFor,
  focusedId,
  cardFor,
  crewOpen = false,
  currentId,
  className,
}: StaysOnRowProps) {
  if (instances.length === 0) return null;
  const ordered = byUrgency(instances);
  const shown = ordered.slice(0, limit);
  const hidden = ordered.length - shown.length;
  return (
    <div
      aria-label="Stays-on agents"
      className={cn("flex min-w-0 items-center gap-1", className)}
      role="toolbar"
    >
      {label}
      {shown.map(instance =>
        instance.id === openMenuFor ? (
          <OpenMenu instance={instance} key={instance.id} surface={surface} />
        ) : (
          <Seat
            cardOpen={instance.id === cardFor || instance.id === focusedId}
            current={instance.id === currentId}
            focused={instance.id === focusedId}
            instance={instance}
            key={instance.id}
            surface={surface}
          />
        )
      )}
      {hidden > 0 ? (
        <CrewPopover
          defaultOpen={crewOpen}
          hidden={hidden}
          instances={instances}
          scopeName={scopeName}
        />
      ) : null}
    </div>
  );
}
