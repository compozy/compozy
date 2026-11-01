/** S10 and S15: the row of main sessions under a project name or in a board header. */
import { Archive, Bot, HeartPulse, Pause, Play } from "lucide-react";
import type { ReactNode } from "react";

import {
  Button,
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
  Tooltip,
  TooltipContent,
  TooltipTrigger,
  cn,
} from "@compozy/ui";

import { AgentAvatar } from "./agent-avatar";
import { AGENT_STATE_LABEL } from "./agent-state";
import { PAUSE_COPY, SCOPE_LABEL, type DraftInstance } from "./fixtures";

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
  size,
}: {
  instance: DraftInstance;
  surface: string;
  size: number;
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

const BUTTON =
  "grid size-8 shrink-0 place-items-center rounded-full outline-none hover:bg-hover focus-visible:shadow-focus-ring";

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
        Heartbeat
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
function OpenMenu({
  instance,
  surface,
  size,
}: {
  instance: DraftInstance;
  surface: string;
  size: number;
}) {
  const paused = instance.state === "paused";
  const blocked =
    instance.pauseReason === "definition_missing" ||
    instance.pauseReason === "definition_not_persistent";
  return (
    <DropdownMenu defaultOpen>
      <DropdownMenuTrigger aria-label={spoken(instance)} className={BUTTON}>
        <Mark instance={instance} size={size} surface={surface} />
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
          Heartbeat
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

function AvatarButton({
  instance,
  surface,
  size,
  focused,
  current,
}: {
  instance: DraftInstance;
  surface: string;
  size: number;
  focused: boolean;
  current: boolean;
}) {
  return (
    <ContextMenu>
      <Tooltip open={focused || undefined}>
        <ContextMenuTrigger
          render={
            <TooltipTrigger
              render={
                <button
                  aria-label={spoken(instance)}
                  aria-current={current ? "page" : undefined}
                  className={cn(BUTTON, focused && "shadow-focus-ring", current && "bg-selected")}
                  type="button"
                />
              }
            />
          }
        >
          <Mark instance={instance} size={size} surface={surface} />
        </ContextMenuTrigger>
        <TooltipContent>{spoken(instance)}</TooltipContent>
      </Tooltip>
      <ContextItems instance={instance} />
    </ContextMenu>
  );
}

function Overflow({ hidden }: { hidden: readonly DraftInstance[] }) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label={`${hidden.length} more persistent agents`}
        render={
          <Button className="shrink-0 rounded-full" size="icon-sm" type="button" variant="quiet" />
        }
      >
        <span className="font-mono text-micro text-muted">+{hidden.length}</span>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start">
        {hidden.map(instance => (
          <DropdownMenuItem key={instance.id}>{spoken(instance)}</DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export interface PersistentRowProps {
  instances: readonly DraftInstance[];
  /** Avatars shown before the rest move into `+N`. */
  limit?: number;
  label?: ReactNode;
  surface?: string;
  size?: number;
  openMenuFor?: string;
  focusedId?: string;
  /** The instance whose main session is open in this window. */
  currentId?: string;
  className?: string;
}

export function PersistentRow({
  instances,
  limit = 6,
  label,
  surface = "ring-sunken",
  size = 24,
  openMenuFor,
  focusedId,
  currentId,
  className,
}: PersistentRowProps) {
  if (instances.length === 0) return null;
  const shown = instances.slice(0, limit);
  const hidden = instances.slice(limit);
  return (
    <div
      aria-label="Persistent agents"
      className={cn("flex min-w-0 items-center gap-0.5", className)}
      role="toolbar"
    >
      {label}
      {shown.map(instance =>
        instance.id === openMenuFor ? (
          <OpenMenu instance={instance} key={instance.id} size={size} surface={surface} />
        ) : (
          <AvatarButton
            current={instance.id === currentId}
            focused={instance.id === focusedId}
            instance={instance}
            key={instance.id}
            size={size}
            surface={surface}
          />
        )
      )}
      {hidden.length > 0 ? <Overflow hidden={hidden} /> : null}
    </div>
  );
}
