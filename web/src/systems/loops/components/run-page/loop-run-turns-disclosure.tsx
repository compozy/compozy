import { ChevronDown } from "lucide-react";
import { Button, Collapsible, CollapsibleContent, CollapsibleTrigger, Spinner } from "@compozy/ui";
import type { GoalTurnsRead } from "../../hooks/use-goal-turns";
import { GoalTurnTimeline } from "./goal-turn-timeline";

export function LoopRunTurnsDisclosure({ read, isLive }: { read: GoalTurnsRead; isLive: boolean }) {
  return (
    <Collapsible className="border-t border-line-soft p-4">
      <CollapsibleTrigger className="group inline-flex min-h-6 items-center gap-1.5 rounded-xs px-1 text-badge font-medium text-muted hover:text-fg-strong focus-visible:outline-none focus-visible:shadow-focus-ring">
        Goal turns
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
          <p role="status" className="mt-3 text-small-body text-muted">
            <Spinner className="mr-2 inline-block size-3" />
            Loading Goal turns…
          </p>
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
            {read.isLoadingMore ? <Spinner className="size-3" /> : null}Load more turns
          </Button>
        ) : null}
      </CollapsibleContent>
    </Collapsible>
  );
}
