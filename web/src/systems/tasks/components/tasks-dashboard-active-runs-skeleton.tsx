import { Panel, Skeleton, SkeletonText } from "@compozy/ui";

import { cn } from "@/lib/utils";

import { RUN_LINE_CLASS, RUN_ROW_CLASS } from "./tasks-dashboard-active-runs-classes";

/** Rows a loading panel stands in with; ids are static, the placeholders carry no identity. */
const LOADING_RUNS = [
  { id: "a", titleWidth: "w-56" },
  { id: "b", titleWidth: "w-44" },
  { id: "c", titleWidth: "w-60" },
] as const;

/**
 * The placeholder twin of the active-runs panel: the same panel head, rows and trailing status,
 * with the counts, titles and times left as blocks. Wrap the dashboard area in a `SkeletonRegion`.
 */
export function TasksDashboardActiveRunsSkeleton() {
  return (
    <Panel
      aria-hidden="true"
      bodyClassName="p-0"
      className="animate-skeleton-reveal"
      right={<SkeletonText className="w-40 eyebrow" />}
      title="Active runs"
    >
      <ul className="divide-y divide-line-soft">
        {LOADING_RUNS.map(run => (
          <li className={RUN_ROW_CLASS} key={run.id}>
            <div className={RUN_LINE_CLASS}>
              <Skeleton className="size-3.5 shrink-0 rounded-full" />
              <div className="flex min-w-0 flex-1 items-baseline gap-2">
                <SkeletonText className={cn("text-small-body", run.titleWidth)} />
                <SkeletonText className="w-16 font-mono text-mono-id" />
              </div>
              <SkeletonText className="w-16 shrink-0" />
              <Skeleton className="size-3.5 shrink-0 rounded-xs" />
            </div>
          </li>
        ))}
      </ul>
    </Panel>
  );
}
