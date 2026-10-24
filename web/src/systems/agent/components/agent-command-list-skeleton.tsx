import { Eyebrow, Skeleton, SkeletonRegion, SkeletonText, cn } from "@compozy/ui";

// The agent picker's list while the catalog has not answered: the groups and rows the loaded list
// is made of, with placeholders where the headings, names, providers and categories go. A row
// repeats what a `CommandItem` is under its default classes, but is none itself, so nothing here
// registers with cmdk or takes the highlight. Static and query-free; the one status region
// carries the loading semantics for the whole list.

/** A `CommandItem` under its default classes: the row each loaded agent is. */
const ROW_FRAME =
  "relative flex min-h-8 w-full min-w-0 items-center gap-2 rounded-md px-2.5 py-1.5 text-small-body";

/** The root group's agents carry no category; a category group's rows trail theirs. */
const GROUPS = [
  {
    id: "root",
    heading: "w-12",
    rows: [
      { id: "a", name: "w-28", category: null },
      { id: "b", name: "w-36", category: null },
      { id: "c", name: "w-24", category: null },
    ],
  },
  {
    id: "category",
    heading: "w-28",
    rows: [
      { id: "a", name: "w-32", category: "w-20" },
      { id: "b", name: "w-24", category: "w-20" },
    ],
  },
] as const;

export function AgentCommandListSkeleton() {
  return (
    <SkeletonRegion data-testid="agent-command-loading" label="Loading agents">
      {GROUPS.map(group => (
        <div aria-hidden="true" className="overflow-hidden p-1" key={group.id}>
          <div className="eyebrow px-2 py-1.5">
            <SkeletonText className={group.heading} />
          </div>
          {group.rows.map(row => (
            <div
              className={cn(ROW_FRAME, "animate-skeleton-reveal")}
              data-slot="agent-command-row-skeleton"
              key={row.id}
            >
              <div className="flex min-w-0 flex-1 items-center gap-2">
                <Skeleton className="size-3 shrink-0 rounded-xs" />
                <SkeletonText className={row.name} />
                <Eyebrow className="shrink-0">
                  <SkeletonText className="w-12" />
                </Eyebrow>
                {row.category === null ? null : (
                  <Eyebrow className="ml-auto shrink-0">
                    <SkeletonText className={row.category} />
                  </Eyebrow>
                )}
              </div>
            </div>
          ))}
        </div>
      ))}
    </SkeletonRegion>
  );
}
