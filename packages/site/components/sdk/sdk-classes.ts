/*
 * Where SDK anchors land. The page scroll padding (128px) clears the 104px docs header; the sticky
 * language bar below it is 67px tall from `sm` and 97px below `sm`, where it wraps onto two rows.
 * Each offset puts the linked title about 21px under the bar.
 */

/** Headings of the SDK guide pages. */
const SDK_HEADING_ANCHOR =
  "[&_h2]:scroll-mt-[94px] [&_h3]:scroll-mt-[94px] sm:[&_h2]:scroll-mt-[64px] sm:[&_h3]:scroll-mt-[64px]";

/** A resource method's section: its title sits below the section's 56px top padding. */
export const SDK_METHOD_ANCHOR = "scroll-mt-[38px] sm:scroll-mt-[8px]";

/** The SDK guide pages space their sections like the draft's SDK template, wider than prose pages. */
export const SDK_PROSE = `[&>h2]:mt-14 [&>h2]:mb-4 [&>div:first-child+h2]:mt-10 ${SDK_HEADING_ANCHOR}`;
