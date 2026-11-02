/** A session's links at rest on a row: a count, with a dot when one needs you. */
import { GitPullRequest } from "@compozy/ui/icons";

import { Pill, Tooltip, TooltipContent, TooltipTrigger } from "@compozy/ui";

import { needsYouLabel, type SessionLinkedWorkSummary } from "../../lib/session-linked-work";

export function SessionLinkCountMark({ summary }: { summary: SessionLinkedWorkSummary }) {
  const { total, needs_you: needsYou } = summary;
  if (total === 0) return null;
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <span
            className="relative z-10 inline-flex shrink-0 items-center gap-1 text-eyebrow text-muted-foreground tabular-nums"
            data-slot="session-link-count"
          />
        }
      >
        <GitPullRequest aria-hidden="true" className="size-3 text-subtle" />
        {total}
        {needsYou > 0 ? <Pill.Dot className="ml-px" tone="brand" /> : null}
      </TooltipTrigger>
      <TooltipContent>
        {total} linked{needsYou > 0 ? ` · ${needsYouLabel(needsYou)}` : ""}
      </TooltipContent>
    </Tooltip>
  );
}
