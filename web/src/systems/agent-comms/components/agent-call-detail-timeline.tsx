/**
 * The call's state timeline, in the rail.
 *
 * Every entry is anchored to a durable timestamp on the record. Where a moment
 * has no timestamp — a call that never started, a call that never settled — the
 * entry simply is not there, rather than appearing greyed with a placeholder
 * time. A timeline that shows a step the runtime never took is worse than a
 * short one.
 */
import { Eyebrow, Time, Timeline, TimelineEvent } from "@compozy/ui";

import type { CallTimelineEvent } from "../lib/call-detail-timeline";

export interface AgentCallDetailTimelineProps {
  events: readonly CallTimelineEvent[];
  "data-testid"?: string;
}

export function AgentCallDetailTimeline({
  events,
  "data-testid": testId,
}: AgentCallDetailTimelineProps) {
  if (events.length === 0) return null;
  return (
    <section data-testid={testId}>
      <Eyebrow className="mb-1.5 block">Timeline</Eyebrow>
      <Timeline ariaLabel="Call state timeline">
        {events.map(event => {
          const Glyph = event.glyph;
          return (
            <TimelineEvent
              key={event.id}
              data-testid={`agent-call-timeline-${event.id}`}
              title={event.title}
              description={event.detail ?? undefined}
              icon={Glyph}
              tone={event.tone === "neutral" ? "neutral" : event.tone}
              time={<Time iso={event.at} mode="absolute" />}
            />
          );
        })}
      </Timeline>
    </section>
  );
}
