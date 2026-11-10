import { cn } from "@compozy/ui";

/**
 * Root type and block rhythm of the reading column, as the draft renders it through the shared
 * Markdown rhythm; the MDX elements in `prose.tsx` add their own look.
 */
export const PROSE = cn(
  "text-[17.5px] leading-[1.75] text-fg/85",
  "[&>*:first-child]:mt-0 [&>*:last-child]:mb-0",
  "[&_p]:my-6 [&_li]:my-2 [&_li>p]:my-2 [&_ol]:my-6 [&_ul]:my-6 [&_strong]:text-fg",
  "[&_:is(h2,h3)+:is(h2,h3)]:mt-3 [&_:is(h2,h3)+:is(p,ol,ul)]:mt-0"
);
