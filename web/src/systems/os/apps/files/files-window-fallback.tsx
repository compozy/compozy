import { SkeletonRegion, SkeletonRows } from "@compozy/ui";

/** The Files window's body while its code loads: the tree's loading rows. */
export function FilesWindowFallback() {
  return (
    <SkeletonRegion label="Loading files" className="p-3" data-testid="files-window-fallback">
      <SkeletonRows count={8} />
    </SkeletonRegion>
  );
}
