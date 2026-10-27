import { Skeleton, SkeletonRegion, SkeletonText, cn } from "@compozy/ui";

import {
  SUBAGENT_ROSTER_LIST_FRAME,
  SUBAGENT_ROSTER_MAIN_FRAME,
  SUBAGENT_ROSTER_MAIN_PLAIN,
  SUBAGENT_ROSTER_ROW_FRAME,
} from "./subagent-roster-frame";

const ROW_POSITIONS = [0, 1, 2, 3, 4, 5] as const;
const TITLE_WIDTHS = ["w-36", "w-28", "w-32", "w-24"] as const;

/**
 * The roster's rows before the list arrives, in the live roster's own list and row frames: the
 * avatar well beside a title, with a short trail. `count` is the rows the session's summary
 * says there are, capped at the roster's first page (six); the one status region of the area.
 */
export function SubagentRosterSkeleton({
  count,
  className,
}: {
  count: number;
  className?: string;
}) {
  const rows = ROW_POSITIONS.slice(0, Math.max(1, Math.min(count, ROW_POSITIONS.length)));
  return (
    <SkeletonRegion
      label="Loading subagents"
      data-testid="subagent-roster-skeleton"
      className={cn(SUBAGENT_ROSTER_LIST_FRAME, className)}
    >
      <ul aria-hidden="true" className="flex flex-col gap-px">
        {rows.map(position => (
          <li key={position} className={SUBAGENT_ROSTER_ROW_FRAME}>
            <div className={cn(SUBAGENT_ROSTER_MAIN_FRAME, SUBAGENT_ROSTER_MAIN_PLAIN)}>
              <Skeleton className="size-6 rounded-full" />
              <SkeletonText
                className={cn("text-transcript-meta", TITLE_WIDTHS[position % TITLE_WIDTHS.length])}
              />
            </div>
            <span className="inline-flex shrink-0 items-center gap-1 text-micro">
              <SkeletonText className="w-8" />
            </span>
          </li>
        ))}
      </ul>
    </SkeletonRegion>
  );
}
