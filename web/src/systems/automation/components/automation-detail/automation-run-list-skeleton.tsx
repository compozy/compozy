import { History } from "lucide-react";

import { Skeleton, SkeletonText } from "@compozy/ui";

import { AutomationDetailSectionSkeleton } from "./automation-detail-section";
import { PANEL_SHELL, RUN_ROW_GRID } from "./automation-run-list-classes";

const RUN_SKELETON_IDS = ["run-skeleton-1", "run-skeleton-2", "run-skeleton-3"] as const;

/** Three run rows in the real row grid: status, what it did, duration, when. */
export function AutomationRunRowsSkeleton() {
  return (
    <ul>
      {RUN_SKELETON_IDS.map(id => (
        <li
          aria-hidden="true"
          className="animate-skeleton-reveal border-t border-line-soft first:border-t-0"
          key={id}
        >
          <div className={RUN_ROW_GRID}>
            <span className="text-small-body">
              <SkeletonText className="w-16" />
            </span>
            <span className="text-small-body">
              <SkeletonText className="w-40" />
            </span>
            <span className="hidden text-form-hint md:block">
              <SkeletonText className="ml-auto w-10" />
            </span>
            <span className="hidden text-form-hint md:block">
              <SkeletonText className="ml-auto w-14" />
            </span>
            <Skeleton className="size-3.5" />
          </div>
        </li>
      ))}
    </ul>
  );
}

/** The Runs section while the page loads: its real label over the panel of placeholder rows. */
export function AutomationRunListSkeleton() {
  return (
    <AutomationDetailSectionSkeleton icon={History} label="Runs">
      <div aria-hidden="true" className={PANEL_SHELL}>
        <AutomationRunRowsSkeleton />
      </div>
    </AutomationDetailSectionSkeleton>
  );
}
