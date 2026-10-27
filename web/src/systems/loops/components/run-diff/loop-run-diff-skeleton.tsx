import {
  Empty,
  ListGroupHeader,
  PillGroup,
  Skeleton,
  SkeletonRegion,
  SkeletonText,
} from "@compozy/ui";
import { GitCompare } from "lucide-react";

import { placeholderSlots } from "../../lib/placeholder-slots";
import { DIFF_GROUP_CLASS, DIFF_ROW_CLASS, DIFF_VIEW_CLASS } from "./loop-run-diff-parts";

const noop = () => undefined;

function DiffGroupSkeleton({ rows }: { rows: number }) {
  return (
    <section aria-hidden="true" className={DIFF_GROUP_CLASS}>
      <ListGroupHeader
        count={<SkeletonText className="w-3" />}
        label={<SkeletonText className="w-16" />}
      />
      <ul>
        {placeholderSlots("diff-row", rows).map(slot => (
          <li className={DIFF_ROW_CLASS} key={slot.id}>
            <Skeleton className="mt-0.5 size-3.5 shrink-0 rounded-full" />
            <div className="min-w-0 flex-1">
              <SkeletonText className="w-32 text-small-body font-medium" />
              <SkeletonText className="mt-1 w-3/4 text-form-hint leading-relaxed" />
              <SkeletonText className="mt-1 w-24 font-mono text-mono-id" />
            </div>
          </li>
        ))}
      </ul>
    </section>
  );
}

/** The comparison's change groups before the diff has been read. Presentational. */
export function LoopRunDiffBodySkeleton() {
  return (
    <div className="flex flex-col gap-4">
      <DiffGroupSkeleton rows={3} />
      <DiffGroupSkeleton rows={2} />
    </div>
  );
}

/** The pickers' real heading and mode switch over select-shaped placeholders for the runs. */
function DiffPickersSkeleton() {
  return (
    <div aria-hidden="true" className="flex flex-wrap items-end gap-x-4 gap-y-3">
      <div className="flex flex-col gap-1.5">
        <span className="eyebrow text-subtle">Compare</span>
        <PillGroup
          aria-label="What to compare against"
          items={[
            { value: "generation", label: "Round", disabled: true },
            { value: "run", label: "Run", disabled: true },
          ]}
          onChange={noop}
          value="generation"
        />
      </div>
      {["Base", "Against"].map(label => (
        <div className="flex min-w-0 flex-col gap-1.5" key={label}>
          <span className="eyebrow text-subtle">{label}</span>
          <Skeleton className="h-control-compact w-full min-w-44 sm:w-44" />
        </div>
      ))}
    </div>
  );
}

/**
 * The compare page before the run has answered: the pickers' heading, mode switch and labels over
 * placeholders, then the diff's groups, or the unselected prompt when the route names no
 * comparison (which is static, so it is real).
 */
export function LoopRunDiffPageSkeleton({ hasComparison }: { hasComparison: boolean }) {
  return (
    <SkeletonRegion data-testid="loop-run-diff-loading" label="Loading comparison">
      <div className={DIFF_VIEW_CLASS}>
        <DiffPickersSkeleton />
        {hasComparison ? <LoopRunDiffBodySkeleton /> : null}
      </div>
      {hasComparison ? null : (
        <Empty
          className="mt-4"
          data-testid="loop-run-diff-unselected"
          description="Pick what to compare with: another round of this run, or another run of this Loop."
          framed
          icon={GitCompare}
          title="Nothing compared yet"
          titleAs="h3"
        />
      )}
    </SkeletonRegion>
  );
}
