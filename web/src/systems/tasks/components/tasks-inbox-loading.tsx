import { ListingPage, Skeleton, SkeletonRegion, SkeletonText } from "@compozy/ui";

import { INBOX_BODY_CLASS, INBOX_TOOLBAR_CLASS } from "./tasks-inbox-classes";
import { TasksInboxRowSkeleton } from "./tasks-inbox-row";

/**
 * Two placeholder groups: the one that waits on a person, with the approval actions, and the
 * quiet feed. The lanes a person has are unknown until the inbox arrives, so no group is named.
 * Ids are static; the placeholders carry no identity of their own.
 */
const LOADING_GROUPS = [
  {
    id: "first",
    rows: [
      { id: "a", titleWidth: "w-56", reason: true, approval: true },
      { id: "b", titleWidth: "w-44", reason: false, approval: true },
      { id: "c", titleWidth: "w-64", reason: true, approval: false },
    ],
  },
  {
    id: "second",
    rows: [
      { id: "a", titleWidth: "w-48", reason: false, approval: false },
      { id: "b", titleWidth: "w-60", reason: false, approval: false },
      { id: "c", titleWidth: "w-40", reason: false, approval: false },
    ],
  },
] as const;

/** The inbox's two loading groups, in the page's own body. One status region. */
export function TasksInboxLoading() {
  return (
    <SkeletonRegion
      className="flex flex-col gap-6"
      data-testid="tasks-inbox-loading"
      label="Loading inbox"
    >
      {LOADING_GROUPS.map(group => (
        <section className="flex flex-col gap-2" key={group.id}>
          <header aria-hidden="true" className="flex items-center gap-2">
            <Skeleton className="size-3.5 shrink-0 rounded-full" />
            <SkeletonText className="w-24 eyebrow" />
            <SkeletonText className="w-4 font-mono text-badge" />
          </header>
          <div className="flex flex-col">
            {group.rows.map(row => (
              <TasksInboxRowSkeleton
                approval={row.approval}
                key={row.id}
                reason={row.reason}
                titleWidth={row.titleWidth}
              />
            ))}
          </div>
        </section>
      ))}
    </SkeletonRegion>
  );
}

/**
 * The inbox before it has answered, without a single live control: the toolbar row as blocks over
 * the loading groups, in the page the loaded inbox fills. Its one status region is the groups'.
 */
export function TasksInboxViewSkeleton() {
  return (
    <ListingPage className="bg-canvas">
      <div aria-hidden="true" className={INBOX_TOOLBAR_CLASS}>
        <Skeleton className="h-search w-64 max-w-full rounded-md" />
        <Skeleton className="h-button-sm w-16 rounded-pill" />
        <div className="ml-auto inline-flex items-center gap-2">
          <Skeleton className="h-switch-default w-switch-default rounded-pill" />
          <SkeletonText className="w-20 eyebrow" />
        </div>
      </div>
      <div className={INBOX_BODY_CLASS}>
        <TasksInboxLoading />
      </div>
    </ListingPage>
  );
}
