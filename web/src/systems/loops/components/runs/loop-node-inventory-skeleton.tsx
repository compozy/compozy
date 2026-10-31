import { ChevronRight } from "@compozy/ui/icons";

import {
  CatalogCardSkeleton,
  ListingToolbar,
  Skeleton,
  SkeletonRegion,
  SkeletonText,
  type ListingViewMode,
} from "@compozy/ui";

import { LOOP_NODE_INVENTORY_LABELS } from "../../lib/loop-node-inventory";
import { placeholderSlots } from "../../lib/placeholder-slots";
import type { LoopNodeInventoryState } from "../../types";
import {
  INVENTORY_CARDS_CLASS,
  INVENTORY_FOOT_CLASS,
  INVENTORY_ROW_CLASS,
  INVENTORY_ROWS_CLASS,
} from "./loop-node-inventory-classes";
import { InventoryHeadRow } from "./loop-node-inventory-parts";
import { LoopNodeInventorySection } from "./loop-node-inventory-section";

const SKELETON_ROWS = 5;

function InventoryRowsSkeleton() {
  return (
    <div className={INVENTORY_ROWS_CLASS}>
      <InventoryHeadRow />
      {placeholderSlots("inventory-row", SKELETON_ROWS).map(slot => (
        <div
          aria-hidden="true"
          className={`${INVENTORY_ROW_CLASS} ${slot.position > 0 ? "border-t border-line-soft" : ""}`}
          key={slot.id}
        >
          <span className="flex min-w-0 flex-1 items-start gap-2">
            <Skeleton className="mt-0.5 size-3.5 shrink-0 rounded-full" />
            <span className="min-w-0 flex-1">
              <SkeletonText className="w-40 text-ws-name" />
              <SkeletonText className="mt-0.5 w-56 text-small-body" />
            </span>
          </span>
          <div className="hidden min-w-0 flex-1 lg:block">
            <SkeletonText className="w-28 text-small-body" />
          </div>
          <div className="hidden min-w-0 flex-1 md:block">
            <SkeletonText className="w-40 text-small-body" />
          </div>
          <div className="w-24 shrink-0">
            <SkeletonText className="ml-auto w-12 font-mono text-mono-id" />
          </div>
          <ChevronRight aria-hidden="true" className="size-3.5 shrink-0 text-faint" />
        </div>
      ))}
      <div aria-hidden="true" className={INVENTORY_FOOT_CLASS}>
        <SkeletonText className="w-32 font-mono text-pill-group-badge" />
      </div>
    </div>
  );
}

function InventoryCardsSkeleton() {
  return (
    <div className={INVENTORY_CARDS_CLASS}>
      <CatalogCardSkeleton actions={false} count={SKELETON_ROWS}>
        <div className="flex min-w-0 flex-col gap-3">
          <span className="flex items-center gap-2">
            <Skeleton className="size-3.5 shrink-0 rounded-full" />
            <SkeletonText className="min-w-0 flex-1 text-ws-name" />
            <SkeletonText className="w-14 shrink-0 text-eyebrow" />
          </span>
          <SkeletonText className="text-small-body" lines={2} />
          <SkeletonText className="w-3/4 text-small-body" />
        </div>
      </CatalogCardSkeleton>
    </div>
  );
}

/**
 * One inventory view before its first read: the rows card (real column header, placeholder rows
 * and foot) or the card grid, matching the view the toolbar has selected.
 */
export function LoopNodeInventorySkeleton({
  state,
  view,
}: {
  state: LoopNodeInventoryState;
  view: ListingViewMode;
}) {
  return (
    <SkeletonRegion
      data-testid="loop-node-inventory-loading"
      label={`Loading ${LOOP_NODE_INVENTORY_LABELS[state].toLowerCase()} steps`}
    >
      {view === "cards" ? <InventoryCardsSkeleton /> : <InventoryRowsSkeleton />}
    </SkeletonRegion>
  );
}

/**
 * The inventory page before its first read, for a window that shows it before the page's code has
 * loaded: the section and its toolbar row (the state switch, the two filters and the view toggle)
 * as blocks, over the results' skeleton. Nothing in it takes focus; the results' status region is
 * the page's only one.
 */
export function LoopNodeInventoryPageSkeleton({
  state,
  view,
}: {
  state: LoopNodeInventoryState;
  view: ListingViewMode;
}) {
  return (
    <LoopNodeInventorySection>
      <ListingToolbar aria-hidden="true">
        <ListingToolbar.Leading>
          <Skeleton className="h-control-compact w-80 shrink-0 rounded-pill" />
          <Skeleton className="h-control-compact w-48 shrink-0 rounded-md" />
          <Skeleton className="h-control-compact w-48 shrink-0 rounded-md" />
        </ListingToolbar.Leading>
        <ListingToolbar.Trailing>
          <Skeleton className="h-control-compact w-32 rounded-pill" />
        </ListingToolbar.Trailing>
      </ListingToolbar>
      <LoopNodeInventorySkeleton state={state} view={view} />
    </LoopNodeInventorySection>
  );
}
