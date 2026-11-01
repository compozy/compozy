/** One place an agent works: what it's doing there right now, with pause and resume at hand. */
import { Archive, MessagesSquare, MoreHorizontal, Pause, Play } from "@compozy/ui/icons";
import { useState } from "react";

import {
  Button,
  ConfirmDialog,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  StatusDot,
  Surface,
  cn,
} from "@compozy/ui";

import { agentCanResume, agentPlaceStatus, agentPlaceTimes } from "../../lib/agent-profile";
import { AGENT_STATE_DOT, type AgentTeamPlace } from "../../lib/agent-team";
import { AgentProjectMark } from "../agent-add-pickers";

export interface AgentPlaceActions {
  onOpen: (place: AgentTeamPlace) => void;
  onToggle: (place: AgentTeamPlace) => void;
  onArchive: (place: AgentTeamPlace) => void;
}

export function AgentPlaceCard({
  place,
  onOpen,
  onToggle,
  onArchive,
}: AgentPlaceActions & { place: AgentTeamPlace }) {
  const [archiving, setArchiving] = useState(false);
  const paused = place.instance.state === "paused";
  const resumable = agentCanResume(place.pauseReason);
  const live = !paused && (place.state === "working" || place.state === "needs-you");
  const { label } = place;
  const dot = AGENT_STATE_DOT[place.state];
  return (
    <Surface
      className={cn(
        "flex min-w-0 flex-col gap-3",
        place.state === "needs-you" && "shadow-inset-strong"
      )}
      data-testid={`agent-place-${place.scope}`}
      render={<article />}
    >
      <div className="flex min-w-0 items-center gap-2.5">
        <AgentProjectMark name={label} />
        <h3 className="min-w-0 flex-1 truncate text-item-title font-medium text-fg-strong">
          {label}
        </h3>
        <Button
          aria-label={`Open the session in ${label}`}
          onClick={() => onOpen(place)}
          size="icon-sm"
          type="button"
          variant="quiet"
        >
          <MessagesSquare aria-hidden="true" />
        </Button>
        <DropdownMenu>
          <DropdownMenuTrigger
            aria-label={`More for ${label}`}
            render={<Button size="icon-sm" type="button" variant="quiet" />}
          >
            <MoreHorizontal aria-hidden="true" />
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onClick={() => setArchiving(true)}>
              <Archive aria-hidden="true" />
              Remove from {label}…
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
      <div className="flex flex-col gap-1">
        <p
          className={cn(
            "flex items-start gap-2 text-small-body",
            live ? "text-fg" : "text-muted-foreground"
          )}
        >
          {dot ? <StatusDot className="mt-1.25 shrink-0" size="sm" tone={dot} /> : null}
          <span className="min-w-0">{agentPlaceStatus(place)}</span>
        </p>
        <p className="pl-3.5 text-eyebrow text-subtle">{agentPlaceTimes(place)}</p>
      </div>
      <div className="mt-auto flex min-h-7 items-end justify-end gap-3">
        {paused ? (
          resumable ? (
            <Button onClick={() => onToggle(place)} size="sm" type="button" variant="secondary">
              <Play aria-hidden="true" />
              Resume
            </Button>
          ) : (
            <span className="text-eyebrow text-subtle">Can't resume</span>
          )
        ) : (
          <Button onClick={() => onToggle(place)} size="sm" type="button" variant="quiet">
            <Pause aria-hidden="true" />
            Pause
          </Button>
        )}
      </div>
      <ConfirmDialog
        cancelLabel="Keep"
        confirmLabel="Remove"
        description="It stops working here. Its sessions and what it remembers are kept."
        onConfirm={() => {
          setArchiving(false);
          onArchive(place);
        }}
        onOpenChange={setArchiving}
        open={archiving}
        title={`Remove from ${label}?`}
      />
    </Surface>
  );
}
