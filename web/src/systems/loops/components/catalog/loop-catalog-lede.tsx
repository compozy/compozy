import { LoopPageLede } from "../loop-page-lede";

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
      lede="Reusable, guardrailed cycles that pursue a goal until it is verified."
      meta={[count, ...(workspaceLabel ? [workspaceLabel] : [])]}
      name="Loops"
      tags={[]}
      testId="loop-catalog-lede"
    />
  );
}
