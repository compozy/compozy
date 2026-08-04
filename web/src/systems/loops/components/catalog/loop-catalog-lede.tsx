interface LoopCatalogLedeProps {
  /** Server-owned total for the active filter set, not the loaded-page length. */
  total: number;
  workspaceLabel?: string;
}

/**
 * Catalog lede: what a loop is, then how many this workspace has.
 *
 * The count is the server total the catalog page reports, so it stays honest while
 * more pages load. Nothing here is derived from the rows on screen.
 */
export function LoopCatalogLede({ total, workspaceLabel }: LoopCatalogLedeProps) {
  return (
    <div className="flex flex-col gap-1 pt-4" data-testid="loop-catalog-lede">
      <h1 className="text-detail-h1 font-medium tracking-detail-h1 text-fg-strong">Loops</h1>
      <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-small-body text-subtle">
        <span>Reusable, guardrailed cycles that pursue a goal until it is verified.</span>
        <span aria-hidden="true">·</span>
        <span data-testid="loop-catalog-count">
          {total} {total === 1 ? "loop" : "loops"}
        </span>
        {workspaceLabel ? (
          <>
            <span aria-hidden="true">·</span>
            <span>{workspaceLabel}</span>
          </>
        ) : null}
      </div>
    </div>
  );
}
