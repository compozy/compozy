/**
 * "A lot happened while you were away": the night at a glance, then only what deserves reading.
 * Quiet check-ins stay a texture; whatever needs you stays a full card below.
 */
import { ChevronRight, CircleAlert, ShieldAlert, Sparkles } from "@compozy/ui/icons";

import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  Separator,
  Timeline,
  TimelineEvent,
  cn,
} from "@compozy/ui";

import { AgentAvatar } from "../stays-on-agents-shared/agent-avatar";
import { TRIAGE } from "../stays-on-agents-shared/fixtures";

type Beat = "quiet" | "reported" | "failed" | "needs-you";

const NOTABLE: Partial<Record<number, Beat>> = {
  3: "reported",
  10: "failed",
  14: "failed",
  21: "reported",
  36: "needs-you",
};

/** One slot per quarter hour from 23:00 to 08:30. */
const NIGHT: readonly Beat[] = Array.from({ length: 38 }, (_, slot) => NOTABLE[slot] ?? "quiet");

const HOURS = ["23:00", "02:00", "05:00", "08:00"];

const BEAT: Record<Beat, string> = {
  quiet: "bg-viz-other",
  reported: "bg-muted-foreground",
  failed: "bg-destructive",
  "needs-you": "bg-brand",
};

const KEY: readonly { beat: Beat; label: string; count: number }[] = [
  { beat: "needs-you", label: "Needs you", count: 1 },
  { beat: "reported", label: "Worth knowing", count: 2 },
  { beat: "failed", label: "Went wrong", count: 2 },
  { beat: "quiet", label: "Nothing to report", count: 33 },
];

function NightStrip() {
  return (
    <div className="flex flex-col gap-1.5">
      <div
        aria-label="Check-ins from 23:00 to 08:30: 1 needs you, 2 worth knowing, 2 went wrong, 33 with nothing to report"
        className="flex h-6 items-stretch gap-0.5"
        role="img"
      >
        {NIGHT.map((beat, index) => (
          <span
            aria-hidden="true"
            className={cn("min-w-0 flex-1 rounded-xs", BEAT[beat])}
            key={index}
          />
        ))}
      </div>
      <div aria-hidden="true" className="flex justify-between font-mono text-micro text-faint">
        {HOURS.map(hour => (
          <span key={hour}>{hour}</span>
        ))}
      </div>
    </div>
  );
}

function Legend() {
  return (
    <ul className="flex flex-wrap gap-x-5 gap-y-1.5">
      {KEY.map(item => (
        <li
          className="flex items-center gap-1.5 text-small-body text-muted-foreground"
          key={item.beat}
        >
          <span aria-hidden="true" className={cn("size-2 rounded-xs", BEAT[item.beat])} />
          {item.label}
          <span className="font-medium text-fg tabular-nums">{item.count}</span>
        </li>
      ))}
    </ul>
  );
}

const EVENTS = [
  {
    icon: CircleAlert,
    tone: "destructive" as const,
    title: "A check ran out of time, twice",
    description: "Both stopped after 10 minutes. The next one finished fine.",
    time: "01:30",
  },
  {
    icon: ShieldAlert,
    tone: "warning" as const,
    title: "A request went unanswered",
    description: "Code Engineer's make migrate wasn't allowed. Nobody was around.",
    time: "03:40",
  },
  {
    icon: Sparkles,
    tone: "neutral" as const,
    title: "Reviewer finished the diff for #931",
    description: "Nothing would break.",
    time: "04:12",
  },
];

/** The reports that came in, read as one story. Whatever needs you follows it as its own card. */
export function AwayDigest() {
  return (
    <section aria-label="While you were away" className="flex flex-col gap-4">
      <Separator label="While you were away" />
      <div className="flex flex-col gap-4 rounded-lg bg-card p-5 shadow-card">
        <div className="flex items-center gap-3.5">
          <AgentAvatar appearance={TRIAGE.appearance} label="Triage Lead" quietIdle size={48} />
          <div className="flex min-w-0 flex-col gap-0.5">
            <h3 className="text-detail-h1 font-medium text-fg-strong">
              Nine quiet hours, one decision for you.
            </h3>
            <p className="text-body text-muted-foreground">
              Triage Lead checked in 38 times. Most had nothing to report.
            </p>
          </div>
        </div>
        <NightStrip />
        <Legend />
        <Collapsible>
          <CollapsibleTrigger className="group flex items-center gap-1.5 rounded-sm text-small-body text-muted-foreground outline-none hover:text-fg focus-visible:shadow-focus-ring">
            <ChevronRight
              aria-hidden="true"
              className="size-3.5 text-faint transition-transform duration-base group-data-[panel-open]:rotate-90 motion-reduce:transition-none"
            />
            What went wrong, and what's worth knowing
          </CollapsibleTrigger>
          <CollapsibleContent>
            <Timeline ariaLabel="Overnight reports" className="gap-3 pt-3">
              {EVENTS.map(event => (
                <TimelineEvent {...event} key={event.title} />
              ))}
            </Timeline>
          </CollapsibleContent>
        </Collapsible>
      </div>
    </section>
  );
}
