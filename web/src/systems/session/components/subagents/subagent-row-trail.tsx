import type { MouseEvent } from "react";
import { ArrowUpRight } from "lucide-react";

import { Icon, cn } from "@compozy/ui";

import { SubagentElapsed } from "./subagent-elapsed";
import { SUBAGENT_STATUS_WORD, subagentElapsedClock } from "./subagent-format";
import {
  SUBAGENT_PR_GLYPH,
  SUBAGENT_PR_TONE,
  subagentPullRequestAriaLabel,
  subagentPullRequestLabel,
} from "./subagent-worktree-format";
import type { SubagentPullRequestView, SubagentView } from "./types";

const PR_LINK_SIZE = {
  card: "h-5 pr-1.5 pl-1.25 text-mono-id",
  row: "h-4.5 pr-1.25 pl-1 text-micro",
} as const;

// The link sits above a row's stretched open control; its own click must not open the session.
const stopOpen = (event: MouseEvent<HTMLAnchorElement>) => event.stopPropagation();

/**
 * `#731` with its state glyph (S5 trail). Opens the PR externally. Always a
 * sibling of the row's open control, never nested in it (Gap 2). `card`
 * reveals an arrow-up-right on hover/focus to signal "external".
 */
export function SubagentPullRequestLink({
  pr,
  size = "card",
  className,
}: {
  pr: SubagentPullRequestView;
  size?: keyof typeof PR_LINK_SIZE;
  className?: string;
}) {
  return (
    <a
      href={pr.url}
      target="_blank"
      rel="noreferrer"
      aria-label={subagentPullRequestAriaLabel(pr)}
      data-slot="subagent-pr-link"
      data-pr={pr.state}
      onClick={stopOpen}
      className={cn(
        "group/pr-link relative z-10 inline-flex shrink-0 items-center gap-1 rounded-pill font-mono font-medium text-fg-2 tabular-nums outline-none transition-colors duration-fast ease-out hover:bg-selected hover:text-fg-strong focus-visible:shadow-focus-ring",
        PR_LINK_SIZE[size],
        className
      )}
    >
      <Icon
        as={SUBAGENT_PR_GLYPH[pr.state]}
        size="sm"
        aria-hidden="true"
        className={SUBAGENT_PR_TONE[pr.state]}
      />
      {subagentPullRequestLabel(pr)}
      {size === "card" ? (
        <Icon
          as={ArrowUpRight}
          size="xs"
          aria-hidden="true"
          className="-ml-px text-faint opacity-0 transition-opacity duration-fast ease-out group-hover/pr-link:opacity-100 group-focus-visible/pr-link:opacity-100 motion-reduce:transition-none"
        />
      ) : null}
    </a>
  );
}

/**
 * Compact row trailing slot (chip preview, inspector roster): elapsed while
 * running or once completed, otherwise the status word — `Failed` in danger,
 * `Waiting for you` in full ink.
 */
export function SubagentRowTrail({
  subagent,
  stale = false,
  className,
}: {
  subagent: SubagentView;
  stale?: boolean;
  className?: string;
}) {
  const clock = subagentElapsedClock(subagent, { stale });
  if ((subagent.status === "running" || subagent.status === "completed") && clock.kind !== "none") {
    return <SubagentElapsed clock={clock} className={className} />;
  }
  return (
    <span
      className={cn(
        "shrink-0 text-transcript-caption whitespace-nowrap",
        subagent.status === "failed"
          ? "text-danger"
          : subagent.status === "waiting"
            ? "text-fg"
            : "text-subtle",
        className
      )}
    >
      {SUBAGENT_STATUS_WORD[subagent.status]}
    </span>
  );
}
