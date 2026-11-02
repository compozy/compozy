/**
 * "A lot happened while you were away": the night at a glance, then only what deserves reading.
 * Quiet check-ins stay a texture; whatever needs you stays a full card below.
 */
import { ChevronRight, CircleAlert, ShieldAlert, Sparkles } from "@compozy/ui/icons";

import {
  AgentAvatar,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  Separator,
  Timeline,
  TimelineEvent,
  cn,
} from "@compozy/ui";

import { PROFILE_SPRITE_URL } from "@/systems/profiles/lib/profile-identity";

import type { AwayBeat, AwayDigestView, AwayEvent } from "../../../lib/session-away";
import type { ReportAgent } from "../../../lib/session-report-identity";

const BEAT: Record<AwayBeat, string> = {
  quiet: "bg-viz-other",
  reported: "bg-muted-foreground",
  failed: "bg-destructive",
  "needs-you": "bg-brand",
};

const EVENT_ICON = { failed: CircleAlert, unanswered: ShieldAlert, reported: Sparkles };
const EVENT_TONE = { failed: "destructive", unanswered: "warning", reported: "neutral" } as const;

function NightStrip({ digest }: { digest: AwayDigestView }) {
  return (
    <div className="flex flex-col gap-1.5">
      <div aria-label={digest.stripLabel} className="flex h-6 items-stretch gap-0.5" role="img">
        {digest.beats.map((beat, index) => (
          <span
            aria-hidden="true"
            className={cn("min-w-0 flex-1 rounded-xs", BEAT[beat])}
            key={index}
          />
        ))}
      </div>
      <div aria-hidden="true" className="flex justify-between font-mono text-micro text-faint">
        {digest.hours.map(hour => (
          <span key={hour}>{hour}</span>
        ))}
      </div>
    </div>
  );
}

function Legend({ digest }: { digest: AwayDigestView }) {
  return (
    <ul className="flex flex-wrap gap-x-5 gap-y-1.5">
      {digest.key.map(item => (
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

function Event({ event }: { event: AwayEvent }) {
  return (
    <TimelineEvent
      description={event.description}
      icon={EVENT_ICON[event.tone]}
      time={event.time}
      title={event.title}
      tone={EVENT_TONE[event.tone]}
    />
  );
}

/** The reports that came in, read as one story. Whatever needs you follows it as its own card. */
export function AwayDigest({ agent, digest }: { agent: ReportAgent; digest: AwayDigestView }) {
  return (
    <section aria-label="While you were away" className="flex flex-col gap-4">
      <Separator label="While you were away" />
      <div className="flex flex-col gap-4 rounded-lg bg-card p-5 shadow-card">
        <div className="flex items-center gap-3.5">
          <AgentAvatar
            appearance={agent.appearance}
            label={agent.label}
            quietIdle
            size={48}
            spriteUrl={PROFILE_SPRITE_URL}
          />
          <div className="flex min-w-0 flex-col gap-0.5">
            <h3 className="text-detail-h1 font-medium text-fg-strong">{digest.headline}</h3>
            <p className="text-body text-muted-foreground">{digest.summary}</p>
          </div>
        </div>
        <NightStrip digest={digest} />
        <Legend digest={digest} />
        {digest.events.length > 0 ? (
          <Collapsible>
            <CollapsibleTrigger className="group flex items-center gap-1.5 rounded-sm text-small-body text-muted-foreground outline-none hover:text-fg focus-visible:shadow-focus-ring">
              <ChevronRight
                aria-hidden="true"
                className="size-3.5 text-faint transition-transform duration-base group-data-[panel-open]:rotate-90 motion-reduce:transition-none"
              />
              What went wrong, and what's worth knowing
            </CollapsibleTrigger>
            <CollapsibleContent>
              <Timeline ariaLabel="Reports while you were away" className="gap-3 pt-3">
                {digest.events.map(event => (
                  <Event event={event} key={event.id} />
                ))}
              </Timeline>
            </CollapsibleContent>
          </Collapsible>
        ) : null}
      </div>
    </section>
  );
}
