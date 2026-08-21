import type { ComponentProps } from "react";
import { ChevronDown } from "lucide-react";

import { Button, cn, Empty, MonoId, Time } from "@compozy/ui";

import type { LoopStoryBeat } from "../../../lib/loop-run-story-beats";

/** Which durable timeline view produced these beats. */
export type LoopEventsView = "notable" | "all";

/**
 * The Events lane's own durable read, separate from Story's notable one.
 *
 * One object rather than four loose flags: they describe a single timeline
 * query, and they are only ever meaningful together.
 */
export interface LoopRunEventsRead {
  /** Read-only: the lane maps the shared projection, it never rewrites it. */
  beats: readonly LoopStoryBeat[];
  /**
   * `all` is the raw activity union the escape hatch is for. `notable` means
   * the page has not wired the raw view yet and the lane is borrowing Story's
   * projection — which it must say out loud rather than imply completeness.
   */
  view: LoopEventsView;
  /** Older durable history is available behind the timeline's backward cursor. */
  hasOlder: boolean;
  isLoading: boolean;
  isError: boolean;
  isLoadingOlder: boolean;
  onLoadOlder?: () => void;
}

interface LoopRunEventsLaneProps extends Omit<ComponentProps<"div">, "children"> {
  read: LoopRunEventsRead;
}

/**
 * The escape hatch, paged from the durable timeline.
 *
 * This is the one lane where sequence numbers and event kinds belong on screen:
 * an operator here is correlating with `compozy loop events` or a log, and the
 * wire vocabulary is the thing they are matching against. Everywhere else on the
 * page those values stay out of sight.
 *
 * History is the daemon's, not a client buffer's: older events page backward
 * through the fenced cursor, so a reload never costs the beginning of a run.
 */
export function LoopRunEventsLane({ read, className, ...props }: LoopRunEventsLaneProps) {
  const { beats, view, hasOlder, isLoading, isError, isLoadingOlder, onLoadOlder } = read;
  if (beats.length === 0) {
    return (
      <Empty
        data-testid="loop-run-events-empty"
        description={
          isError
            ? "This run's events could not be read. The run itself is unaffected."
            : isLoading
              ? "Reading this run's events…"
              : "Nothing has been recorded for this run yet."
        }
        title={isError ? "Events unavailable" : "No events"}
      />
    );
  }
  return (
    <div className={cn("flex flex-col", className)} data-view={view} {...props}>
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
      {view === "notable" ? (
        <p
          className="border-t border-line-soft px-4 py-2 text-form-hint text-faint"
          data-testid="loop-run-events-notable-only"
        >
          Notable beats only — raw activity is not being read for this lane.
        </p>
      ) : null}
      {hasOlder && onLoadOlder ? (
        <div className="border-t border-line-soft px-2 py-1.5">
          <Button
            className="w-full justify-center"
            data-testid="loop-run-events-load-older"
            disabled={isLoadingOlder}
            onClick={onLoadOlder}
            size="sm"
            type="button"
            variant="ghost"
          >
            <ChevronDown aria-hidden="true" />
            {isLoadingOlder ? "Loading earlier events…" : "Load earlier events"}
          </Button>
        </div>
      ) : null}
    </div>
  );
}
