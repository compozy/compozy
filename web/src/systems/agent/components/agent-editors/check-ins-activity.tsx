/** S7's right column: what happens next, how today went, and Try it now. */
import { Check, MessageCircle, Moon, Play, TriangleAlert, X } from "@compozy/ui/icons";
import { useState } from "react";

import {
  Button,
  CommandEmpty,
  CommandItem,
  CommandList,
  CommandSelect,
  CommandSelectGroup,
  CommandSelectShell,
  CommandSelectTrigger,
  Eyebrow,
  Spinner,
  Timeline,
  TimelineEvent,
} from "@compozy/ui";

import type { CheckInsEditorModel } from "../../hooks/use-check-ins-editor";
import type { TryCheck } from "../../hooks/use-agent-instance-checks";
import { gapLabel } from "../../lib/agent-check-in-activity";
import { dayCells, type NextCheckIn } from "../../lib/agent-check-in-form";
import { agentScopeLabel } from "../../lib/agent-instance";
import type { AgentInstancePayload } from "../../types";
import { DayStrip, DayStripKey, EditorCard } from "./rich-controls";

function PlacePicker({
  place,
  places,
  onChange,
}: {
  place: AgentInstancePayload;
  places: readonly AgentInstancePayload[];
  onChange: (next: string) => void;
}) {
  const [open, setOpen] = useState(false);
  return (
    <CommandSelect onOpenChange={setOpen} open={open}>
      <CommandSelectTrigger
        aria-label="Project"
        className="h-7 w-auto max-w-44 gap-1 border-0 bg-transparent px-2 hover:bg-hover"
        label={agentScopeLabel(place.scope)}
        selected
      />
      <CommandSelectShell align="end" inputPlaceholder="Find a project">
        <CommandList>
          <CommandEmpty>No project found</CommandEmpty>
          <CommandSelectGroup>
            {places.map(item => (
              <CommandItem
                key={item.id}
                onSelect={() => {
                  onChange(item.id);
                  setOpen(false);
                }}
                value={agentScopeLabel(item.scope)}
              >
                {agentScopeLabel(item.scope)}
              </CommandItem>
            ))}
          </CommandSelectGroup>
        </CommandList>
      </CommandSelectShell>
    </CommandSelect>
  );
}

function Banner({ children, onClose }: { children: React.ReactNode; onClose: () => void }) {
  return (
    <div className="flex items-center gap-2 rounded-md bg-sunken p-3 text-small-body text-fg-2">
      {children}
      <Button aria-label="Close" onClick={onClose} size="icon-xs" type="button" variant="quiet">
        <X aria-hidden="true" />
      </Button>
    </div>
  );
}

function thingsHeading(count: number) {
  if (count === 0) return "Nothing to look into right now";
  return `It would look into ${count} ${count === 1 ? "thing" : "things"}`;
}

function TryResult({
  state,
  onRunForReal,
  onClose,
}: {
  state: Exclude<TryCheck, { phase: "idle" }>;
  onRunForReal: () => void;
  onClose: () => void;
}) {
  if (state.phase === "running") {
    return (
      <div
        className="flex items-center gap-2 rounded-md bg-sunken p-3 text-small-body text-muted-foreground"
        role="status"
      >
        <Spinner className="size-3.5" />
        Looking around…
      </div>
    );
  }
  if (state.phase === "unavailable" || state.phase === "stopped") {
    return (
      <Banner onClose={onClose}>
        <span className="min-w-0 flex-1">
          {state.phase === "stopped" ? state.sentence : "This can't run here yet."}
        </span>
      </Banner>
    );
  }
  if (state.phase === "sent") {
    return (
      <Banner onClose={onClose}>
        <Check aria-hidden="true" className="size-3.5 shrink-0 text-success" />
        <span className="min-w-0 flex-1">{state.sentence}</span>
      </Banner>
    );
  }
  return (
    <div className="flex flex-col gap-3 rounded-md bg-sunken p-3">
      <div className="flex items-center gap-2">
        <span className="min-w-0 flex-1 text-small-body font-medium text-fg">
          {thingsHeading(state.digest.length)}
        </span>
        <Button aria-label="Close" onClick={onClose} size="icon-xs" type="button" variant="quiet">
          <X aria-hidden="true" />
        </Button>
      </div>
      {state.digest.length > 0 ? (
        <ul className="flex list-disc flex-col gap-1 ps-4 text-small-body text-fg-2">
          {state.digest.map(item => (
            <li key={item.key}>{item.summary}</li>
          ))}
        </ul>
      ) : null}
      <p className="text-eyebrow text-subtle">That was only a try. Nothing was sent.</p>
      <Button onClick={onRunForReal} size="sm" type="button" variant="secondary">
        Run it for real
      </Button>
    </div>
  );
}

function stoppedReason({ standing, failedInARow, pauseCopy, instance }: CheckInsEditorModel) {
  if (standing === "failed") {
    const count = failedInARow > 0 ? `${failedInARow} check-ins` : "Too many check-ins";
    return `${count} in a row failed, so it stopped trying.`;
  }
  const manual = "You paused it. It waits until you start it again.";
  return instance?.pause_reason === "manual" ? manual : (pauseCopy ?? manual);
}

function StoppedStatement({ model }: { model: CheckInsEditorModel }) {
  const failed = model.standing === "failed";
  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-col gap-0.5">
        <span className="text-display-2xl font-medium text-fg">
          {failed ? "Stopped" : "Paused"}
        </span>
        <span className="text-small-body text-muted-foreground">{stoppedReason(model)}</span>
      </div>
      <Button
        className="self-start"
        disabled={model.resumeBlocked}
        onClick={model.resume}
        size="sm"
        type="button"
      >
        {failed ? "Start again" : "Resume"}
      </Button>
    </div>
  );
}

function StandingStatement({
  model,
  next,
}: {
  model: CheckInsEditorModel;
  next: NextCheckIn | null;
}) {
  if (model.standing === "paused" || model.standing === "failed") {
    return <StoppedStatement model={model} />;
  }
  return (
    <div className="flex flex-col gap-0.5">
      <span className="text-display-2xl font-medium text-fg tabular-nums">
        {next ? `${next.day} at ${next.time}` : "None planned"}
      </span>
      <span className="text-small-body text-muted-foreground">
        {next ? (next.distance ?? "When its hours start") : "Check-ins are off"}
      </span>
    </div>
  );
}

export function NextCard({ model }: { model: CheckInsEditorModel }) {
  const { standing, settings, next, activity, nowHour, instance, places, tryCheck } = model;
  const running = standing === "running" || standing === "slowed";
  const past = activity?.past ?? [];
  const { cells, done, total } = dayCells(settings, past, nowHour, running);
  const told = past.filter(item => item.cell === "told").length;
  return (
    <EditorCard className="gap-5">
      <div className="-my-1 flex items-center justify-between gap-2">
        <Eyebrow>{running ? "Next check-in" : "Check-ins"}</Eyebrow>
        {instance ? (
          <PlacePicker onChange={model.setPlace} place={instance} places={places} />
        ) : null}
      </div>
      <StandingStatement model={model} next={running ? next : null} />
      {model.slowedBy !== null ? (
        <p className="flex gap-2 rounded-md bg-sunken p-3 text-small-body text-fg-2">
          <Moon aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-muted-foreground" />
          {`Nothing has changed for a while, so it's checking every ${gapLabel(model.slowedBy)}. It speeds back up when you send it a message.`}
        </p>
      ) : null}
      <div className="flex flex-col gap-2.5">
        <div className="flex items-baseline justify-between gap-2 text-small-body text-muted-foreground tabular-nums">
          <span>
            <span className="font-medium text-fg">{done}</span> of {total} today
          </span>
          {told > 0 ? <span>{told} worth telling you</span> : null}
        </div>
        <DayStrip cells={cells} label={`${done} of ${total} check-ins done today`} />
        <div className="flex flex-wrap gap-x-3 gap-y-1">
          {standing === "failed" ? (
            <DayStripKey cell="failed">Failed</DayStripKey>
          ) : (
            <>
              <DayStripKey cell="quiet">All quiet</DayStripKey>
              <DayStripKey cell="told">Told you</DayStripKey>
              {running ? <DayStripKey cell="next">Next</DayStripKey> : null}
            </>
          )}
        </div>
      </div>
      {tryCheck.state.phase === "idle" ? (
        <Button
          onClick={() => void tryCheck.tryIt()}
          type="button"
          variant={running ? "default" : "secondary"}
        >
          <Play aria-hidden="true" />
          Try it now
        </Button>
      ) : (
        <TryResult
          onClose={tryCheck.reset}
          onRunForReal={() => void tryCheck.runForReal()}
          state={tryCheck.state}
        />
      )}
    </EditorCard>
  );
}

export function RecentCard({ activity }: { activity: CheckInsEditorModel["activity"] }) {
  return (
    <EditorCard className="gap-4">
      <h2 className="text-item-title font-medium text-fg">Today so far</h2>
      {activity === null ? (
        <p className="text-small-body text-muted-foreground">
          Check-ins will show up here once they start.
        </p>
      ) : activity.recent.length === 0 ? (
        <p className="text-small-body text-muted-foreground">Nothing yet today.</p>
      ) : (
        <Timeline ariaLabel="Recent check-ins">
          {activity.recent.map(item => (
            <TimelineEvent
              description={item.description}
              icon={item.failed ? TriangleAlert : item.told ? MessageCircle : undefined}
              key={item.key}
              time={item.time}
              title={item.title}
              tone={item.failed ? "destructive" : "neutral"}
            />
          ))}
        </Timeline>
      )}
    </EditorCard>
  );
}
