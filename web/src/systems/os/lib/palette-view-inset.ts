/**
 * One left edge for a pushed palette view.
 *
 * Surfaces that are themselves a box (search field, chips, rows) share
 * `px-2` on top of the Command frame. Glyphs that sit in the open
 * (back icon, footer keys) use `px-4` so they land on the same line as
 * the search icon and the row mark — CommandInput and CommandItem each
 * add their own inner `px-2`.
 */
export const paletteViewFrameClass = "p-2";

export const paletteViewGutterClass = "px-2";

export const paletteViewLeadClass = "px-4";

export const paletteViewFieldClass =
  "[&_[data-slot=command-input-wrapper]]:px-2 [&_[data-slot=command-input-wrapper]]:pt-2 [&_[data-slot=command-input-wrapper]]:pb-2";

export const paletteViewListClass = "max-h-[46vh] px-2 pt-1 pb-4";

export const paletteViewItemClass = "px-2 py-2.5";
