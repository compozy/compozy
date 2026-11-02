/** A session's links at rest on a row or card: a count, with a dot when one needs you. */
import { GitPullRequest } from "@compozy/ui/icons";

import { Pill, Tooltip, TooltipContent, TooltipTrigger } from "@compozy/ui";

import type { SessionLink } from "@/systems/session/mocks/session-links-fixtures";

import { needsYouCount, needsYouLabel } from "./link-model";

export function LinkCountMark({ links }: { links: readonly SessionLink[] }) {
  const needsYou = needsYouCount(links);
  if (links.length === 0) return null;
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
        {links.length}
        {needsYou > 0 ? <Pill.Dot className="ml-px" tone="brand" /> : null}
      </TooltipTrigger>
      <TooltipContent>
        {links.length} linked{needsYou > 0 ? ` · ${needsYouLabel(needsYou)}` : ""}
      </TooltipContent>
    </Tooltip>
  );
}
