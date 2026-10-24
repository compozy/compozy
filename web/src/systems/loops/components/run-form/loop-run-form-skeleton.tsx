import { FolderGit2, Gauge, TextCursorInput } from "lucide-react";

import { PageContent, Skeleton, SkeletonRegion, SkeletonText } from "@compozy/ui";

import { placeholderSlots } from "../../lib/placeholder-slots";
import { LoopPageLedeSkeleton } from "../loop-page-lede";
import { LoopRailSectionSkeleton } from "../loop-rail-section";
import { LoopRunFormActions } from "./loop-run-form-actions";
import {
  IDLE_ACTION_NOTE,
  RUN_FORM_PAGE_CLASS,
  RUN_FORM_STACK_CLASS,
} from "./loop-run-form-layout";

const noop = () => undefined;
const GIST = <SkeletonText className="ml-auto w-24" />;

interface LoopRunFormSkeletonProps {
  /** The Loop's name, known from the route before any read answers. */
  name: string;
}

/**
 * The run form before the Loop, its settings and the live run have answered: the form's own
 * frame, the lede with its name already printed, the sections' real headings, and the action
 * bar with its buttons waiting. The window head is published by the location, so it is already
 * whole.
 */
export function LoopRunFormSkeleton({ name }: LoopRunFormSkeletonProps) {
  return (
    <SkeletonRegion
      className="flex min-h-0 flex-1 flex-col"
      data-testid="loop-run-form-loading"
      label="Loading run form"
    >
      <div className="flex min-h-0 flex-1 flex-col overflow-y-auto">
        <PageContent className={RUN_FORM_PAGE_CLASS} density="route">
          <div className={RUN_FORM_STACK_CLASS}>
            <LoopPageLedeSkeleton lede={2} name={name} prefix="Run" tags={2} />
            <LoopRailSectionSkeleton
              gist={GIST}
              icon={<TextCursorInput aria-hidden="true" className="size-3.5" />}
              open
              title="Inputs"
            >
              <div className="flex flex-col gap-4 px-4 py-3">
                {placeholderSlots("input", 2).map(slot => (
                  <div className="flex flex-col gap-1.5" key={slot.id}>
                    <SkeletonText className="w-28 text-form-label" />
                    <Skeleton className="h-input w-full" />
                    <SkeletonText className="w-3/5 text-form-hint" />
                  </div>
                ))}
              </div>
            </LoopRailSectionSkeleton>
            <LoopRailSectionSkeleton
              gist={GIST}
              icon={<FolderGit2 aria-hidden="true" className="size-3.5" />}
              title="Environment"
            />
            <LoopRailSectionSkeleton
              gist={GIST}
              icon={<Gauge aria-hidden="true" className="size-3.5" />}
              title="Limits"
            />
            <LoopRunFormActions
              busy
              note={IDLE_ACTION_NOTE}
              onDryRun={noop}
              onRun={noop}
              pendingKind={null}
              profileDestination={null}
              valid={false}
            />
          </div>
        </PageContent>
      </div>
    </SkeletonRegion>
  );
}
