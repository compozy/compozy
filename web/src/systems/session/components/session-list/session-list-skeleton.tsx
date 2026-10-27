import { Skeleton, SkeletonRegion, SkeletonText, cn } from "@compozy/ui";

import { SESSION_LIST_BAR_CLASS } from "../../lib/session-list-bar-class";
import { SESSION_SIDEBAR_MIN_WIDTH } from "../../lib/session-sidebar-width";
import { BRANCH_NODE, BRANCH_RAIL } from "./session-list-branch-rail";
import {
  SESSION_ROW_END_SLOT_FRAME,
  SESSION_ROW_FRAME,
  SESSION_ROW_META_FRAME,
  SESSION_ROW_TARGET_FRAME,
  SESSION_ROW_TRAILING_FRAME,
  SESSION_ROW_WHO_FRAME,
  SUBAGENT_ROW_FRAME,
} from "./session-list-row-frame";

// Content-shaped placeholders for the sessions rail. A row repeats the live row's frame
// (`session-list-row-frame`) with `SkeletonText` where the title, the time and the agent go, so
// it is exactly as tall as a real row. Every shape is presentational: whoever renders a list
// of them wraps the area in the one `SkeletonRegion` that carries the loading semantics.

/** The scrolling body of the rail, without its scroll: placeholders end where the rail does. */
const SESSION_LIST_BODY_FRAME =
  "flex min-h-0 flex-1 flex-col gap-0.5 overflow-hidden px-2.5 pt-0.5";

const TITLE_WIDTHS = ["w-40", "w-28", "w-36", "w-48", "w-32"] as const;
const AGENT_WIDTHS = ["w-16", "w-20", "w-12", "w-24"] as const;
/** Placeholders have no identity, so a row's position is its key and what its widths vary by. */
const ROW_POSITIONS = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11] as const;

function pick<T>(widths: readonly T[], position: number): T {
  return widths[position % widths.length];
}

/**
 * One placeholder session row: status mark, title and time on line 1, provider and agent on
 * line 2. `branch` adds the branch toggle's place at the end of line 2, as a thread's root has.
 */
function SessionRowSkeleton({ position, branch = false }: { position: number; branch?: boolean }) {
  return (
    <div aria-hidden="true" data-slot="session-list-row-skeleton" className={SESSION_ROW_FRAME}>
      <span className={SESSION_ROW_TARGET_FRAME}>
        <span className="grid size-3 shrink-0 place-items-center">
          <Skeleton className="size-2.5 rounded-full" />
        </span>
        <SkeletonText className={pick(TITLE_WIDTHS, position)} />
      </span>
      <span className={SESSION_ROW_END_SLOT_FRAME}>
        <span className="font-mono text-micro">
          <SkeletonText className="w-6" />
        </span>
      </span>
      <span className={SESSION_ROW_META_FRAME}>
        <span className={SESSION_ROW_WHO_FRAME}>
          <Skeleton className="size-3 shrink-0 rounded-xs" />
          <SkeletonText className={pick(AGENT_WIDTHS, position)} />
        </span>
        {branch ? (
          <span className={SESSION_ROW_TRAILING_FRAME}>
            <Skeleton className="h-5 w-10 rounded-pill" />
          </span>
        ) : null}
      </span>
    </div>
  );
}

/** `count` placeholder rows. Siblings, so put them where the live rows go. */
function SessionRowsSkeleton({ count = 3, offset = 0 }: { count?: number; offset?: number }) {
  return ROW_POSITIONS.slice(offset, offset + count).map(position => (
    <SessionRowSkeleton key={position} position={position} />
  ));
}

/**
 * The flat list while its first page loads: a root, a root with two child sessions hanging off
 * its rail by the live tree elbows, then more roots, enough to fill the rail.
 */
function SessionThreadsSkeleton() {
  return (
    <>
      <SessionRowsSkeleton count={2} />
      <SessionRowSkeleton position={2} branch />
      {/* The live elbows are borders, not blocks: the rail rides one reveal so they do not appear alone. */}
      <div aria-hidden="true" className={cn(BRANCH_RAIL, "animate-skeleton-reveal")}>
        <div className={BRANCH_NODE}>
          <SessionRowSkeleton position={3} />
        </div>
        <div className={BRANCH_NODE}>
          <SessionRowSkeleton position={4} />
        </div>
      </div>
      <SessionRowsSkeleton count={6} offset={5} />
    </>
  );
}

/** The list body while the catalog loads: the one status region, in the body's own frame. */
export function SessionListSkeleton({ className }: { className?: string }) {
  return (
    <SkeletonRegion
      label="Loading sessions"
      data-testid="session-list-skeleton"
      className={cn(SESSION_LIST_BODY_FRAME, className)}
    >
      <SessionThreadsSkeleton />
    </SkeletonRegion>
  );
}

/** A group header at rest: chevron, name and count, with the group's own create control. */
function SessionGroupHeaderSkeleton({ position }: { position: number }) {
  return (
    <div aria-hidden="true" className="flex min-h-7 min-w-0 items-center gap-1">
      <div className="flex min-h-7 min-w-0 flex-1 items-center gap-2 rounded-sm px-2 py-1">
        <Skeleton className="size-3 shrink-0 rounded-xs" />
        <span className="flex min-w-0 flex-1 items-center text-small-body">
          <SkeletonText className={pick(TITLE_WIDTHS, position + 1)} />
        </span>
        <span className="font-mono text-micro">
          <SkeletonText className="w-4" />
        </span>
      </div>
      <Skeleton className="size-button-icon-xs shrink-0 rounded-sm" />
    </div>
  );
}

/** A group's rows while its page loads, under the group's own header, which is already real. */
export function SessionGroupRowsSkeleton() {
  return (
    <SkeletonRegion
      label="Loading sessions"
      data-testid="session-list-group-skeleton"
      className="flex flex-col gap-0.5 pl-2.5"
    >
      <SessionRowsSkeleton count={3} />
    </SkeletonRegion>
  );
}

/** The groups themselves while their facets load: headers, each with its first rows. */
export function SessionGroupsSkeleton() {
  return (
    <SkeletonRegion
      label="Loading sessions"
      data-testid="session-list-groups-skeleton"
      className="flex flex-col gap-1"
    >
      <SessionGroupHeaderSkeleton position={0} />
      <div className="flex flex-col gap-0.5 pl-2.5">
        <SessionRowsSkeleton count={3} />
      </div>
      <SessionGroupHeaderSkeleton position={2} />
      <div className="flex flex-col gap-0.5 pl-2.5">
        <SessionRowsSkeleton count={2} offset={3} />
      </div>
    </SkeletonRegion>
  );
}

/** A subagent row on its parent's rail at rest: the live row's frame with a status mark, title and trail. */
function SessionSubagentRowSkeleton({ position }: { position: number }) {
  return (
    <div className={BRANCH_NODE} aria-hidden="true">
      <div className={SUBAGENT_ROW_FRAME}>
        <span className="flex justify-center">
          <Skeleton className="size-2 rounded-full" />
        </span>
        <SkeletonText className={pick(TITLE_WIDTHS, position)} />
        <span className="font-mono text-micro">
          <SkeletonText className="w-6" />
        </span>
      </div>
    </div>
  );
}

/** `count` subagent rows on their parent's rail while its list loads, under one status region. */
export function SessionSubagentRowsSkeleton({ count }: { count: number }) {
  return (
    <SkeletonRegion label="Loading subagents">
      {ROW_POSITIONS.slice(0, count).map(position => (
        <SessionSubagentRowSkeleton key={position} position={position} />
      ))}
    </SkeletonRegion>
  );
}

/**
 * The sessions rail of a window whose session has not loaded: the rail's own shell, toolbar
 * strip and filter at their real heights, over placeholder rows. Static on purpose (no queries),
 * so a restored desktop of hidden session windows stays cheap while each one loads. Rendered
 * inside the window's one status region.
 */
export function SessionSidebarSkeleton() {
  return (
    <aside
      aria-hidden="true"
      data-testid="session-sidebar-skeleton"
      className="flex h-full w-full flex-col overflow-hidden bg-sunken"
    >
      <div
        className="flex h-full w-full flex-col pt-1.5"
        style={{ minWidth: SESSION_SIDEBAR_MIN_WIDTH }}
      >
        <div className="flex min-h-0 flex-1 flex-col">
          <div className={SESSION_LIST_BAR_CLASS}>
            <Skeleton className="h-button-sm w-20 rounded-md" />
            <Skeleton className="ml-auto size-button-icon-sm shrink-0 rounded-md" />
            <Skeleton className="size-button-icon-sm shrink-0 rounded-md" />
          </div>
          <div className="px-3 pb-1.5">
            <Skeleton className="h-search w-full rounded-md" />
          </div>
          <div className={SESSION_LIST_BODY_FRAME}>
            <SessionThreadsSkeleton />
          </div>
        </div>
      </div>
    </aside>
  );
}
