import { Skeleton, SkeletonRegion } from "@compozy/ui";

const RAIL_SLOTS = ["sessions", "desktops", "terminal", "agents", "tasks", "loops", "automations"];

/**
 * The desktop while the app route still resolves identity and workspaces: the same grid, menubar
 * row, rail column and inset desk panel as `DesktopShell`, so the real chrome lands in place
 * instead of replacing a blank page. Static and query-free; it renders before any store exists.
 */
export function DesktopShellSkeleton() {
  return (
    <SkeletonRegion
      label="Loading CompozyOS"
      data-testid="os-desktop-loading"
      className="grid min-h-0 flex-1 grid-cols-[auto_minmax(0,1fr)] grid-rows-[auto_minmax(0,1fr)] overflow-hidden bg-rail"
    >
      <div aria-hidden="true" className="col-span-full flex h-menubar items-center gap-4 px-4">
        <Skeleton className="size-5 rounded-md" />
        <Skeleton className="h-3 w-24" />
        <Skeleton className="h-3 w-40" />
      </div>
      <div aria-hidden="true" className="flex w-rail flex-col items-center gap-4 pt-3">
        {RAIL_SLOTS.map(slot => (
          <Skeleton key={slot} className="size-5 rounded-md" />
        ))}
      </div>
      <div
        aria-hidden="true"
        className="min-h-0 overflow-hidden rounded-tl-lg border-t border-l border-line bg-canvas"
      />
    </SkeletonRegion>
  );
}
