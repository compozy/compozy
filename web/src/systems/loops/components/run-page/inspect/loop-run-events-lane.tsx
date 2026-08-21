import { Empty, MonoId, Time } from "@compozy/ui";

import type { LoopStoryBeat } from "../../../lib/loop-run-story-beats";

interface LoopRunEventsLaneProps {
  beats: LoopStoryBeat[];
}

/**
 * The escape hatch, paged from the durable timeline.
 *
 * This is the one lane where sequence numbers and event kinds belong on screen:
 * an operator here is correlating with `compozy loop events` or a log, and the
 * wire vocabulary is the thing they are matching against. Everywhere else on the
 * page those values stay out of sight.
 */
export function LoopRunEventsLane({ beats }: LoopRunEventsLaneProps) {
  if (beats.length === 0) {
    return (
      <Empty
        data-testid="loop-run-events-empty"
        description="Nothing has been recorded for this run yet."
        title="No events"
      />
    );
  }
  return (
    <ul className="flex flex-col divide-y divide-line-soft" data-testid="loop-run-events-lane">
      {beats.map(beat => (
        <li
          className="flex flex-wrap items-baseline gap-x-3 gap-y-1 px-4 py-2"
          data-seq={beat.seq}
          data-testid={`loop-run-event-${beat.seq}`}
          key={beat.key}
        >
          <MonoId size="sm" value={String(beat.seq)} />
          <span className="font-mono text-mono-id text-faint">{beat.kind}</span>
          <span className="min-w-0 flex-1 truncate text-small-body text-fg-strong">
            {beat.title}
          </span>
          {beat.count > 1 ? (
            <span className="font-mono text-mono-id text-faint">×{beat.count}</span>
          ) : null}
          <span className="text-form-hint text-faint">
            <Time iso={beat.at} />
          </span>
        </li>
      ))}
    </ul>
  );
}
