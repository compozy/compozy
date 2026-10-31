import { ChevronDown } from "@compozy/ui/icons";
import {
  Button,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  Eyebrow,
  Skeleton,
  SkeletonRegion,
  SkeletonText,
  Spinner,
} from "@compozy/ui";
import { placeholderSlots } from "../../lib/placeholder-slots";
import type { GoalTurnsRead } from "../../hooks/use-goal-turns";
import { GoalTurnTimeline } from "./goal-turn-timeline";

/** Exposes paginated Goal history with loading, retry, and retained-data states. */
export function LoopRunTurnsDisclosure({ read, isLive }: { read: GoalTurnsRead; isLive: boolean }) {
  return (
    <Collapsible className="border-t border-line-soft p-4">
      <CollapsibleTrigger className="group inline-flex min-h-6 items-center gap-1.5 rounded-xs px-1 text-badge font-medium text-muted hover:text-fg-strong focus-visible:outline-none focus-visible:shadow-focus-ring">
        <Eyebrow className="text-muted">Goal turns</Eyebrow>
        <span className="font-mono text-mono-id tabular-nums text-faint">
          {read.turns.length}
          {read.hasMore ? "+" : ""}
        </span>
        <ChevronDown
          aria-hidden="true"
          className="size-3 transition-transform group-data-[state=closed]:-rotate-90"
        />
      </CollapsibleTrigger>
      <CollapsibleContent>
        {read.isError ? (
          <div role="alert" className="mt-3 text-small-body text-danger">
            Could not load Goal turns.{" "}
            {read.turns.length > 0 ? "Showing the last available history." : ""}
            {read.onRetry ? (
              <Button onClick={read.onRetry} size="sm" variant="ghost">
                Try again
              </Button>
            ) : null}
          </div>
        ) : null}
        {read.isLoading ? (
          <SkeletonRegion className="mt-3 flex flex-col gap-3" label="Loading Goal turns">
            {placeholderSlots("turn", 2).map(slot => (
              <div aria-hidden="true" className="flex items-start gap-3" key={slot.id}>
                <Skeleton className="mt-0.5 size-4 shrink-0 rounded-full" />
                <div className="min-w-0 flex-1">
                  <SkeletonText className="w-40 text-small-body" />
                  <SkeletonText className="mt-1 w-3/4 text-form-label" />
                </div>
              </div>
            ))}
          </SkeletonRegion>
        ) : null}
        {read.turns.length > 0 || (!read.isLoading && !read.isError) ? (
          <GoalTurnTimeline turns={read.turns} live={isLive} className="mt-3" />
        ) : null}
        {read.hasMore && read.onLoadMore ? (
          <Button
            className="mt-2"
            disabled={read.isLoadingMore}
            onClick={read.onLoadMore}
            size="sm"
            variant="ghost"
          >
            {read.isLoadingMore ? <Spinner className="size-3.5" /> : null}Load more turns
          </Button>
        ) : null}
      </CollapsibleContent>
    </Collapsible>
  );
}
