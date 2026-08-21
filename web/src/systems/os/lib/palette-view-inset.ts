/**
 * Palette spatial grammar: one 20px left rail, 32px command rows.
 *
 * Command keeps `p-1` (4px). List `px-1` + item `px-3` lands the search
 * glyph, group labels, and row marks on the same edge. Footer keys and
 * breadcrumb marks use `px-4` inside that frame. Do not add a second
 * frame pad — that is what used to split the rail into three edges.
 *
 * Zones stay distinct: a `--line` hairline closes the query head, then
 * the results well uses the same 12px inset as its bottom edge. Row
 * height is compact; the chrome around the list is not.
 */

export const paletteInputRailClass = "[&_[data-slot=command-input-group]]:px-3";

/** Query head: breadcrumb + field, closed by the same hairline as the footer. */
export const paletteHeadClass = "border-b border-line pb-2";

export const paletteListClass = "max-h-[46vh] px-1 pt-3 pb-3";

export const paletteGroupClass =
  "p-0 **:[[cmdk-group-heading]]:px-3 **:[[cmdk-group-heading]]:py-1 **:[[cmdk-group-heading]]:text-faint";

export const paletteGroupFollowClass = "mt-2 border-t border-line pt-2";

export const paletteRowClass = "mt-0.5 h-control-compact gap-2 px-3 py-0 leading-none first:mt-0";

export const paletteRowTwoLineClass = "mt-0.5 min-h-11 gap-2 px-3 py-1.5 leading-none first:mt-0";

/** Glyphs that sit in the open (back icon, breadcrumb, footer keys). */
export const paletteViewLeadClass = "px-4";

/** Boxed chrome that is not a CommandItem (filter chips). */
export const paletteViewGutterClass = "px-4";

/** Root and pushed views share Command `p-1`. Do not override it with `p-2`. */
export const paletteViewFrameClass = "";

export const paletteViewFieldClass = paletteInputRailClass;

export const paletteViewListClass = paletteListClass;

export function paletteItemClass(twoLine = false): string {
  return twoLine ? paletteRowTwoLineClass : paletteRowClass;
}

export function paletteRowEstimate(twoLine = false): number {
  return twoLine ? 44 : 32;
}

export const paletteViewItemClass = paletteRowClass;
