// Layout classes the marketplace pages and their loading shapes share, so a placeholder cannot
// drift from the card or page it stands in for. In a module of their own that imports nothing, so
// a loading shape does not bring the rest of the marketplace helpers with it.

/**
 * The row-card layout every catalog and Installed card shares: logo · text · trail. The entry card
 * and its loading skeleton both take it, so a placeholder card cannot drift from the real one.
 */
export const MARKETPLACE_ENTRY_CARD_CLASS =
  "grid min-h-15 grid-cols-[var(--size-provider-logo-well)_minmax(0,1fr)_auto] items-center gap-3 px-3 py-2.5";

/** The column both Browse and Installed fill inside their listing page; also a container for the grid. */
export const MARKETPLACE_PAGE_BODY_CLASS = "@container flex min-w-0 flex-col gap-3.5";

/** Installed lists one card per row from 960px, with the description held to two lines. */
export const MARKETPLACE_INSTALLED_GRID_CLASS =
  "@min-[960px]:grid-cols-1 [&_[data-slot=catalog-card-description]]:line-clamp-2";
