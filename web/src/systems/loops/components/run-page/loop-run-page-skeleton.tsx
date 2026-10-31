import { Coins, Gauge, History, Info, Search } from "@compozy/ui/icons";

import {
  Button,
  cn,
  Eyebrow,
  PropertyRowSkeleton,
  Skeleton,
  SkeletonRegion,
  SkeletonText,
} from "@compozy/ui";

import { LoopSectionSkeleton } from "../loop-section";
import { LoopRunPageFrame } from "./loop-run-page-frame";
import {
  BRIEFING_FRAME_CLASS,
  PROGRESS_CARD_CLASS,
  PROGRESS_HEADING_CLASS,
  PROGRESS_META_CLASS,
  RAIL_CARD_CLASS,
  STORY_CARD_CLASS,
  WEIGHT_CLASS,
} from "./loop-run-page-classes";
import { LoopRunStepRowsSkeleton } from "./loop-run-step-rows-skeleton";
import { LoopRunStoryRowsSkeleton } from "./loop-run-story-rows-skeleton";

/**
 * The briefing strip before the run's verdict has been read: the calm strip's frame with a glyph,
 * a headline and a detail line. Presentational; the caller owns the `SkeletonRegion`.
 */
export function LoopRunBriefingSkeleton() {
  return (
    <section
      aria-hidden="true"
      className={cn(BRIEFING_FRAME_CLASS, WEIGHT_CLASS.calm)}
      data-testid="loop-run-briefing-loading"
    >
      <Skeleton className="mt-1 size-4 shrink-0 rounded-full" />
      <div className="min-w-0 flex-1">
        <SkeletonText className="w-3/5 text-item-title font-medium tracking-tight" />
        <SkeletonText className="mt-1 w-full max-w-[68ch] text-small-body leading-relaxed" />
      </div>
    </section>
  );
}

/**
 * The Progress section before the briefing's counts have been read: the real heading over the
 * card's goal line, bar, counts and a few step rows. Presentational.
 */
export function LoopRunStepsProgressSkeleton() {
  return (
    <LoopSectionSkeleton
      aria-hidden="true"
      data-testid="loop-run-progress-loading"
      icon={<Gauge aria-hidden="true" />}
      title="Progress"
    >
      <div className={PROGRESS_CARD_CLASS}>
        <div className="p-4">
          <SkeletonText className={cn("w-2/3", PROGRESS_HEADING_CLASS)} />
          <Skeleton className="mt-4 h-1.5 w-full rounded-pill" />
          <div className={PROGRESS_META_CLASS}>
            <SkeletonText className="w-28" />
            <SkeletonText className="w-16" />
          </div>
          <LoopRunStepRowsSkeleton />
        </div>
      </div>
    </LoopSectionSkeleton>
  );
}

/** The activity section before the timeline has answered: its heading and card of beats. */
export function LoopRunStorySkeleton() {
  return (
    <LoopSectionSkeleton aria-hidden="true" icon={<History aria-hidden="true" />} title="Activity">
      <div className={STORY_CARD_CLASS}>
        <LoopRunStoryRowsSkeleton />
      </div>
    </LoopSectionSkeleton>
  );
}

/** The folded Inspect section: its real heading and closed chevron, a gist still unread. */
function LoopRunInspectSkeleton() {
  return (
    <LoopSectionSkeleton
      aria-hidden="true"
      icon={<Search aria-hidden="true" />}
      open={false}
      title="Inspect"
    />
  );
}

/** The rail's usage, about and Inspect rows: static headings over placeholder values. */
function LoopRunRailSkeleton() {
  return (
    <div aria-hidden="true" className={RAIL_CARD_CLASS}>
      <div className="px-4 py-4">
        <div className="mb-2 flex items-center gap-1.5">
          <Coins aria-hidden="true" className="size-3 text-muted" />
          <Eyebrow className="text-subtle">Usage</Eyebrow>
        </div>
        <PropertyRowSkeleton count={4} />
        <SkeletonText className="mt-2.5 text-badge leading-normal" lines={2} />
      </div>
      <div className="border-t border-line-soft px-4 py-4">
        <div className="flex w-full items-center gap-1.5 text-left">
          <Info aria-hidden="true" className="size-3 text-muted" />
          <Eyebrow className="text-subtle">About this run</Eyebrow>
        </div>
      </div>
      <div className="flex items-center border-t border-line-soft px-3 py-2">
        <Button disabled size="sm" tabIndex={-1} type="button" variant="ghost">
          <Search aria-hidden="true" />
          Inspect
        </Button>
      </div>
    </div>
  );
}

/**
 * The run page before the run record has answered: the page's own frame, with the briefing,
 * progress, activity and Inspect sections and the rail's usage and about card as placeholders
 * where their reads will land. It is the same set the body shows while its own reads are
 * pending, so the run record arriving moves nothing.
 */
export function LoopRunPageSkeleton() {
  return (
    <SkeletonRegion
      className="flex min-h-0 flex-1 flex-col"
      data-testid="loop-run-detail-loading"
      label="Loading run"
    >
      <LoopRunPageFrame
        main={
          <>
            <LoopRunBriefingSkeleton />
            <LoopRunStepsProgressSkeleton />
            <LoopRunStorySkeleton />
            <LoopRunInspectSkeleton />
          </>
        }
        rail={<LoopRunRailSkeleton />}
      />
    </SkeletonRegion>
  );
}
