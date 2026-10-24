import { Skeleton, SkeletonRegion, SkeletonText } from "@compozy/ui";

/**
 * The New tab window before its code has loaded: the same centered column the loaded window is
 * (the empty-tab line, the destination button and the palette chord line), as blocks under one
 * busy status. Static, with no store, so a restored desktop of tabs loads cheaply.
 */
export function NewTabWindowSkeleton() {
  return (
    <SkeletonRegion
      className="flex flex-1 flex-col items-center justify-center gap-3 p-8 text-center"
      data-testid="new-tab-loading"
      label="Loading new tab"
    >
      <SkeletonText className="w-28 text-small-body leading-relaxed" />
      <Skeleton className="h-button-sm w-40 rounded-pill" />
      <SkeletonText className="w-36 text-small-body" />
    </SkeletonRegion>
  );
}
