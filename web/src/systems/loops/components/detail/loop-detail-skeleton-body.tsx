import {
  ChevronRight,
  Gauge,
  History,
  ScrollText,
  TextCursorInput,
  Workflow,
  Zap,
} from "@compozy/ui/icons";

import { Skeleton, SkeletonRegion, SkeletonText } from "@compozy/ui";

import { placeholderSlots } from "../../lib/placeholder-slots";
import { LoopPageLedeSkeleton } from "../loop-page-lede";
import { LoopRailSectionSkeleton } from "../loop-rail-section";
import { LoopSectionSkeleton } from "../loop-section";
import { LoopBodyDagSkeleton } from "./loop-body-dag-skeleton";
import { ContractRow } from "./loop-contract-row";
import { LoopDetailFrame } from "./loop-detail-frame";
import { LoopRecentRunsSkeleton } from "./loop-recent-runs-skeleton";

const GIST = <SkeletonText className="ml-auto w-24" />;

/**
 * The goal card as most Loops fill it: a goal line, a finish line and the folded details. A Loop
 * that also lists verification checks is taller than this, which is the one part the skeleton
 * cannot know before the definition arrives.
 */
function ContractSkeleton() {
  return (
    <LoopSectionSkeleton icon={<ScrollText aria-hidden="true" />} title="Goal and finish line">
      <div className="flex flex-col rounded-lg bg-card shadow-card">
        <ContractRow label="Goal">
          <SkeletonText className="text-small-body leading-relaxed" />
        </ContractRow>
        <ContractRow label="Done when">
          <SkeletonText className="text-small-body leading-relaxed" />
        </ContractRow>
        <div className="border-t border-line-soft">
          <div className="flex w-full items-center gap-1.5 px-4 py-3 text-left text-form-hint text-subtle">
            <ChevronRight aria-hidden="true" className="size-3 shrink-0" />
            Details
          </div>
        </div>
      </div>
    </LoopSectionSkeleton>
  );
}

function InputsRailSkeleton() {
  return (
    <LoopRailSectionSkeleton
      gist={GIST}
      icon={<TextCursorInput aria-hidden="true" className="size-3.5" />}
      open
      title="Inputs"
    >
      <div className="flex flex-col">
        {placeholderSlots("input", 3).map(slot => (
          <div className="border-t border-line-soft px-4 py-2.5 first:border-t-0" key={slot.id}>
            <SkeletonText className="w-28 text-form-label" />
            <SkeletonText className="mt-1 text-form-hint leading-snug" />
          </div>
        ))}
      </div>
    </LoopRailSectionSkeleton>
  );
}

export interface LoopDetailSkeletonProps {
  /** The Loop's name, known from the route before any read answers. */
  name: string;
}

/**
 * The Loop detail page before its definition and settings have answered: the page's own frame
 * with every static heading real and the data-bearing parts as placeholders in their slots.
 * Presentational: it publishes nothing to the window head and reads no router, so a window can
 * show it before its code has loaded.
 */
export function LoopDetailSkeletonBody({ name }: LoopDetailSkeletonProps) {
  return (
    <SkeletonRegion
      className="flex min-h-0 flex-1 flex-col"
      data-testid="loop-detail-loading"
      label="Loading Loop"
    >
      <LoopDetailFrame
        lede={<LoopPageLedeSkeleton lede={1} meta={2} name={name} tags={2} />}
        main={
          <>
            <ContractSkeleton />
            <LoopSectionSkeleton
              icon={<Workflow aria-hidden="true" />}
              right={<Skeleton className="h-button-sm w-24" />}
              title="Steps"
            >
              <LoopBodyDagSkeleton />
            </LoopSectionSkeleton>
            <LoopSectionSkeleton
              icon={<History aria-hidden="true" />}
              right={<Skeleton className="h-button-sm w-28" />}
              title="Recent runs"
            >
              <LoopRecentRunsSkeleton />
            </LoopSectionSkeleton>
          </>
        }
        rail={
          <>
            <InputsRailSkeleton />
            <LoopRailSectionSkeleton
              gist={GIST}
              icon={<Zap aria-hidden="true" className="size-3.5" />}
              title="Automations"
            />
            <LoopRailSectionSkeleton
              gist={GIST}
              icon={<Gauge aria-hidden="true" className="size-3.5" />}
              title="Limits"
            />
          </>
        }
      />
    </SkeletonRegion>
  );
}
