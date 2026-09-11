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

export const paletteInputRailClass =
  "[&_[data-slot=command-input-group]]:px-3 " +
  // Touch tier (S6/T5): the query field reaches the 44px floor so the palette
  // input is a full thumb target and stays legible above the keyboard.
  "max-[760px]:[&_[data-slot=command-input-group]]:h-auto " +
  "max-[760px]:[&_[data-slot=command-input-group]]:min-h-(--height-button-cta-lg)";

/** Query head: breadcrumb + field, closed by the same hairline as the footer. */
export const paletteHeadClass = "border-b border-line pb-2";

/**
 * Results well: desktop caps at 46vh; the touch tier grows to
 * min(52dvh, 440px) — the palette stays top-anchored at 390×844 (T5), and the
 * well scrolls under the keyboard instead of pushing the input off-screen.
 */
export const paletteListClass = "max-h-[46vh] px-1 pt-3 pb-3 max-[760px]:max-h-[min(52dvh,440px)]";

export const paletteGroupClass =
  "p-0 **:[[cmdk-group-heading]]:px-3 **:[[cmdk-group-heading]]:py-1 **:[[cmdk-group-heading]]:text-faint";

export const paletteGroupFollowClass = "mt-2 border-t border-line pt-2";

/**
 * Palette rows (S6/T1): desktop rows sit at the 32px compact control height;
 * at the ≤760px touch tier every row reaches the shared 44px floor, so the
 * ⌘K palette is thumb-navigable at 390×844.
 */
export const paletteRowClass =
  "mt-0.5 h-control-compact gap-2 px-3 py-0 leading-none first:mt-0 " +
  "max-[760px]:h-auto max-[760px]:min-h-(--height-button-cta-lg)";

export const paletteRowTwoLineClass =
  "mt-0.5 min-h-11 gap-2 px-3 py-1.5 leading-none first:mt-0 " +
  "max-[760px]:min-h-(--height-button-cta-lg)";

/** Glyphs that sit in the open (back icon, breadcrumb, footer keys). */
export const paletteViewLeadClass = "px-4";

/** Boxed chrome that is not a CommandItem (filter chips). */
export const paletteViewGutterClass = "px-4";

export const paletteViewListClass = paletteListClass;

export function paletteItemClass(twoLine = false): string {
  return twoLine ? paletteRowTwoLineClass : paletteRowClass;
}

export function paletteRowEstimate(twoLine = false): number {
  return twoLine ? 44 : 32;
}
