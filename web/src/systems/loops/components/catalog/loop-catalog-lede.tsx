import { LoopPageLede, LoopPageLedeSkeleton } from "../loop-page-lede";

const CATALOG_LEDE = "Repeatable jobs that keep working on a goal until it's done.";

interface LoopCatalogLedeProps {
  /** Server-owned total for the active filter set, not the loaded-page length. */
  total: number;
  workspaceLabel?: string;
}

/**
 * Catalog lede on the shared `LoopPageLede` anatomy (name, meta, lede, `.dh` hairline).
 *
 * The count is the server total the catalog page reports, so it stays honest while
 * more pages load. Nothing here is derived from the rows on screen.
 */
export function LoopCatalogLede({ total, workspaceLabel }: LoopCatalogLedeProps) {
  const count = `${total} ${total === 1 ? "loop" : "loops"}`;
  return (
    <LoopPageLede
      lede={CATALOG_LEDE}
      meta={[count, ...(workspaceLabel ? [workspaceLabel] : [])]}
      name="Loops"
      tags={[]}
      testId="loop-catalog-lede"
    />
  );
}

/** The catalog lede before the first read: heading and sentence are static, the count is not. */
export function LoopCatalogLedeSkeleton({
  workspaceLabel,
}: Pick<LoopCatalogLedeProps, "workspaceLabel">) {
  return (
    <LoopPageLedeSkeleton
      lede={CATALOG_LEDE}
      meta={workspaceLabel ? 2 : 1}
      name="Loops"
      testId="loop-catalog-lede"
    />
  );
}
