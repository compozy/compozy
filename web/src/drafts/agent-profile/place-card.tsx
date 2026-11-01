/** One place an agent works: what it's doing there right now, with pause and resume at hand. */
import { Archive, MessagesSquare, MoreHorizontal, Pause, Play } from "lucide-react";
import { useState } from "react";

import {
  Button,
  ConfirmDialog,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  Sparkline,
  StatusDot,
  cn,
  type StatusDotTone,
} from "@compozy/ui";

import { ProjectMark } from "../persistent-agents-shared/add-pickers";
import type { AgentState } from "../persistent-agents-shared/agent-state";
import { PAUSE_SHORT } from "../persistent-agents-shared/agents-fixtures";
import { SCOPE_LABEL } from "../persistent-agents-shared/fixtures";
import { canResume } from "./place-edits";
import type { ProfilePlace } from "./profile-fixtures";

const DOT: Partial<Record<AgentState, StatusDotTone>> = {
  working: "success",
  failed: "danger",
  paused: "faint",
  idle: "faint",
  "needs-you": "accent",
};

function status(place: ProfilePlace) {
  if (place.pauseReason) return PAUSE_SHORT[place.pauseReason];
  if (place.state === "paused") return "You paused it";
  if (place.now) return place.now;
  return place.state === "working" ? "Working" : "Resting until its next check-in";
}

function Times({ place }: { place: ProfilePlace }) {
  if (place.state === "paused") {
    return <span>Last checked in {place.last ?? "never"}</span>;
  }
  return (
    <span>
      {place.last ? `Checked in ${place.last}` : "No check-ins yet"}
      {place.next ? ` · next ${place.next}` : ""}
    </span>
  );
}

export interface PlaceCardProps {
  place: ProfilePlace;
  readOnly?: boolean;
  onToggle: (scope: string) => void;
  onArchive: (scope: string) => void;
}

export function PlaceCard({ place, onToggle, onArchive }: PlaceCardProps) {
  const [archiving, setArchiving] = useState(false);
  const paused = place.state === "paused";
  const resumable = canResume(place.pauseReason);
  const live = place.state === "working" || place.state === "needs-you";
  const label = SCOPE_LABEL(place.scope);
  const dot = DOT[place.state];
  return (
    <article
      className={cn(
        "flex min-w-0 flex-col gap-3 rounded-lg bg-card p-4 shadow-card",
        place.state === "needs-you" && "shadow-inset-strong"
      )}
    >
      <div className="flex min-w-0 items-center gap-2.5">
        <ProjectMark name={label} />
        <h3 className="min-w-0 flex-1 truncate text-item-title font-medium text-fg">{label}</h3>
        <Button
          aria-label={`Open the session in ${label}`}
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
          className={cn("flex items-start gap-2 text-small-body", live ? "text-fg" : "text-muted")}
        >
          {dot ? <StatusDot className="mt-1.25 shrink-0" size="sm" tone={dot} /> : null}
          <span className="min-w-0">{status(place)}</span>
        </p>
        <p className="pl-3.5 text-eyebrow text-subtle">
          <Times place={place} />
        </p>
      </div>
      <div className="mt-auto flex min-h-7 items-end justify-between gap-3">
        <Sparkline
          ariaLabel={`Activity in ${label} over the last 2 weeks`}
          className="w-32 shrink-0 gap-0.5"
          height={24}
          values={place.activity}
        />
        {paused ? (
          resumable ? (
            <Button
              onClick={() => onToggle(place.scope)}
              size="sm"
              type="button"
              variant="secondary"
            >
              <Play aria-hidden="true" />
              Resume
            </Button>
          ) : (
            <span className="text-eyebrow text-subtle">Can't resume</span>
          )
        ) : (
          <Button onClick={() => onToggle(place.scope)} size="sm" type="button" variant="quiet">
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
          onArchive(place.scope);
        }}
        onOpenChange={setArchiving}
        open={archiving}
        title={`Remove from ${label}?`}
      />
    </article>
  );
}
