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

import { DayStrip, DayStripKey, EditorCard } from "../stays-on-agents-shared/rich-controls";
import {
  dayCells,
  type CheckInSettings,
  type NextCheckIn,
  type PlaceActivity,
  type Standing,
  type TryPhase,
} from "./checkins-model";
import { useTryIt } from "./hooks/use-try-it";

function PlacePicker({
  place,
  places,
  onChange,
}: {
  place: string;
  places: readonly string[];
  onChange: (next: string) => void;
}) {
  const [open, setOpen] = useState(false);
  return (
    <CommandSelect onOpenChange={setOpen} open={open}>
      <CommandSelectTrigger
        aria-label="Project"
        className="h-7 w-auto max-w-44 gap-1 border-0 bg-transparent px-2 hover:bg-hover"
        label={place}
        selected
      />
      <CommandSelectShell align="end" inputPlaceholder="Find a project">
        <CommandList>
          <CommandEmpty>No project found</CommandEmpty>
          <CommandSelectGroup>
            {places.map(item => (
              <CommandItem
                key={item}
                onSelect={() => {
                  onChange(item);
                  setOpen(false);
                }}
                value={item}
              >
                {item}
              </CommandItem>
            ))}
          </CommandSelectGroup>
        </CommandList>
      </CommandSelectShell>
    </CommandSelect>
  );
}

function TryResult({
  phase,
  onRunForReal,
  onClose,
}: {
  phase: TryPhase;
  onRunForReal: () => void;
  onClose: () => void;
}) {
  if (phase === "running") {
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
  if (phase === "sent") {
    return (
      <div className="flex items-center gap-2 rounded-md bg-sunken p-3 text-small-body text-fg-2">
        <Check aria-hidden="true" className="size-3.5 shrink-0 text-success" />
        <span className="min-w-0 flex-1">Done. It told you in its main chat.</span>
        <Button aria-label="Close" onClick={onClose} size="icon-xs" type="button" variant="quiet">
          <X aria-hidden="true" />
        </Button>
      </div>
    );
  }
  return (
    <div className="flex flex-col gap-3 rounded-md bg-sunken p-3">
      <div className="flex items-center gap-2">
        <span className="min-w-0 flex-1 text-small-body font-medium text-fg">
          It would look into 2 things
        </span>
        <Button aria-label="Close" onClick={onClose} size="icon-xs" type="button" variant="quiet">
          <X aria-hidden="true" />
        </Button>
      </div>
      <ul className="flex list-disc flex-col gap-1 ps-4 text-small-body text-fg-2">
        <li>Migrate tokens is stuck</li>
        <li>Nightly triage failed twice</li>
      </ul>
      <p className="text-eyebrow text-subtle">That was only a try. Nothing was sent.</p>
      <Button onClick={onRunForReal} size="sm" type="button" variant="secondary">
        Run it for real
      </Button>
    </div>
  );
}

function StandingStatement({
  standing,
  next,
  onResume,
}: {
  standing: Standing;
  next: NextCheckIn | null;
  onResume: () => void;
}) {
  if (standing === "paused" || standing === "failed") {
    const failed = standing === "failed";
    return (
      <div className="flex flex-col gap-3">
        <div className="flex flex-col gap-0.5">
          <span className="text-display-2xl font-medium text-fg">
            {failed ? "Stopped" : "Paused"}
          </span>
          <span className="text-small-body text-muted-foreground">
            {failed
              ? "10 check-ins in a row failed, so it stopped trying."
              : "You paused it. It waits until you start it again."}
          </span>
        </div>
        <Button className="self-start" onClick={onResume} size="sm" type="button">
          {failed ? "Start again" : "Resume"}
        </Button>
      </div>
    );
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

export function NextCard({
  settings,
  next,
  standing,
  place,
  places,
  activity,
  initialTry,
  onPlace,
  onResume,
}: {
  settings: CheckInSettings;
  next: NextCheckIn | null;
  standing: Standing;
  place: string;
  places: readonly string[];
  activity: PlaceActivity;
  initialTry: TryPhase;
  onPlace: (next: string) => void;
  onResume: () => void;
}) {
  const tryIt = useTryIt(initialTry);
  const running = standing === "running" || standing === "slowed";
  const { cells, done, total } = dayCells(
    running ? settings : { ...settings, on: false },
    activity.past
  );
  const told = activity.past.filter(item => item.cell === "told").length;
  return (
    <EditorCard className="gap-5">
      <div className="-my-1 flex items-center justify-between gap-2">
        <Eyebrow>{running ? "Next check-in" : "Check-ins"}</Eyebrow>
        <PlacePicker onChange={onPlace} place={place} places={places} />
      </div>
      <StandingStatement next={running ? next : null} onResume={onResume} standing={standing} />
      {standing === "slowed" ? (
        <p className="flex gap-2 rounded-md bg-sunken p-3 text-small-body text-fg-2">
          <Moon aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-muted-foreground" />
          Nothing has changed for a while, so it's checking every 2 hours. It speeds back up when
          you send it a message.
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
              {next && running ? <DayStripKey cell="next">Next</DayStripKey> : null}
            </>
          )}
        </div>
      </div>
      {tryIt.phase === "idle" ? (
        <Button
          onClick={() => tryIt.run("result")}
          type="button"
          variant={running ? "default" : "secondary"}
        >
          <Play aria-hidden="true" />
          Try it now
        </Button>
      ) : (
        <TryResult
          onClose={tryIt.reset}
          onRunForReal={() => tryIt.run("sent")}
          phase={tryIt.phase}
        />
      )}
    </EditorCard>
  );
}

export function RecentCard({ activity }: { activity: PlaceActivity }) {
  return (
    <EditorCard className="gap-4">
      <h2 className="text-item-title font-medium text-fg">Today so far</h2>
      <Timeline ariaLabel="Recent check-ins">
        {activity.recent.map(item => (
          <TimelineEvent
            description={item.description}
            icon={item.failed ? TriangleAlert : item.told ? MessageCircle : undefined}
            key={item.time}
            time={item.time}
            title={item.title}
            tone={item.failed ? "destructive" : "neutral"}
          />
        ))}
      </Timeline>
    </EditorCard>
  );
}
