import { ListingPage, SkeletonRegion, type ListingViewMode } from "@compozy/ui";

import {
  ToolbarFilterSkeleton,
  ToolbarSearchSkeleton,
  ToolbarSegmentsSkeleton,
  ToolbarSlotSkeleton,
  ToolbarViewToggleSkeleton,
} from "../../components/os-window-toolbar-skeleton";
import type { OsWindowRoute } from "../../lib/os-types";
import { parseLoopWindowLocation } from "./loop-window-location";
import {
  LoopCatalogSkeleton,
  LoopDetailSkeletonBody,
  LoopEditorStaticSkeleton,
  LoopNodeInventoryPageSkeleton,
  LoopRunDiffPageSkeleton,
  LoopRunFormSkeleton,
  LoopRunPageSkeleton,
  loopRunDiffQuery,
  LoopRunsSkeleton,
  type LoopRunsRouteSearch,
} from "@/systems/loops";

function LoopsCatalogFallback({ view }: { view: ListingViewMode }) {
  return (
    <>
      <ToolbarSlotSkeleton
        leading={
          <>
            <ToolbarSearchSkeleton />
            <ToolbarFilterSkeleton />
          </>
        }
        trailing={<ToolbarViewToggleSkeleton />}
      />
      <ListingPage>
        <LoopCatalogSkeleton view={view} />
      </ListingPage>
    </>
  );
}

/** The runs roster, or the step inventory when the route asks for one. */
function LoopRunsFallback({ search }: { search: LoopRunsRouteSearch }) {
  const inventory = search.nodes;
  return (
    <>
      <ToolbarSlotSkeleton
        leading={
          <>
            <ToolbarSegmentsSkeleton className="w-40" />
            {inventory === undefined ? <ToolbarFilterSkeleton /> : null}
          </>
        }
      />
      <ListingPage>
        {inventory === undefined ? (
          <LoopRunsSkeleton withProject={false} />
        ) : (
          <LoopNodeInventoryPageSkeleton state={inventory} view={search.view ?? "rows"} />
        )}
      </ListingPage>
    </>
  );
}

/**
 * The Loops window's body while its code loads: the surface the route is about to show, in the
 * loading state its location shows while that surface's read is out. The listings publish the
 * strip their location will. The configure route is a dialog over an empty body, so its body is
 * the empty region.
 */
export function LoopsWindowFallback({ route }: { route: OsWindowRoute }) {
  const location = parseLoopWindowLocation(route);
  switch (location.kind) {
    case "run-diff":
      return (
        <ListingPage>
          <LoopRunDiffPageSkeleton hasComparison={loopRunDiffQuery(location.search) !== null} />
        </ListingPage>
      );
    case "run":
      return <LoopRunPageSkeleton />;
    case "runs":
      return <LoopRunsFallback search={location.search} />;
    case "configure":
      return <SkeletonRegion className="min-h-0 flex-1" label="Loading Loop settings" />;
    case "editor":
      return <LoopEditorStaticSkeleton />;
    case "run-form":
      return <LoopRunFormSkeleton name={location.name} />;
    case "detail":
      return <LoopDetailSkeletonBody name={location.name} />;
    case "catalog":
      return <LoopsCatalogFallback view={location.search.view ?? "rows"} />;
  }
}
