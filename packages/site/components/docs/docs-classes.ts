import { cn } from "@compozy/ui";
import { isValidElement, type ReactNode } from "react";

/** Class fragments shared by the docs shell, ported from `web/src/drafts/docs/text.ts`. */
export { BEAM, BEAM_X, FOCUS } from "@/components/site/layout";

export const LINE = "border-site-light-080";
/** The fixed header is 104px tall: 56 for the bar, 48 for the section tabs. */
export const BELOW_HEADER = "top-[104px]";

/** The plain text of a rendered tree: headings, Shiki lines, MDX children. */
export function textOf(node: ReactNode): string {
  if (typeof node === "string" || typeof node === "number") return String(node);
  if (Array.isArray(node)) return node.map(textOf).join("");
  if (isValidElement(node)) return textOf((node.props as { children?: ReactNode }).children);
  return "";
}

/** The body of every docs page; MDX blocks set their own spacing inside it. */
export const DOCS_PROSE = cn(
  "docs-prose text-[16px] leading-[1.7] text-fg/80",
  "[&>p]:my-5 [&_:not(.not-prose)>li]:my-1.5 [&_:not(.not-prose)>li]:pl-0.5 [&_:not(.not-prose)>li]:leading-[1.7] [&_li>p]:my-1.5 [&>ol]:my-5 [&>ul]:my-5 [&_strong]:font-medium [&_strong]:text-fg",
  "[&_:is(ol,ul):not(.not-prose)>li:first-child]:mt-0 [&_:is(ol,ul):not(.not-prose)>li:last-child]:mb-0",
  "[&>p:has(+ol:not(.not-prose))]:mb-2 [&>p:has(+ul:not(.not-prose))]:mb-2 [&>p+ol:not(.not-prose)]:mt-0 [&>p+ul:not(.not-prose)]:mt-0",
  "[&_ul:not(.not-prose)]:list-disc [&_ul:not(.not-prose)]:pl-6 [&_ol:not(.not-prose)]:list-decimal [&_ol:not(.not-prose)]:pl-6 [&_li]:marker:text-subtle",
  "[&>blockquote]:my-7 [&>blockquote]:border-l-2 [&>blockquote]:border-site-light-250 [&>blockquote]:pl-5 [&>blockquote]:text-muted-foreground",
  "[&>h4]:mt-5 [&>h4]:mb-1 [&>h4]:text-[16.5px] [&>h4]:font-medium [&>h4]:text-fg",
  "[&>hr]:my-8 [&>hr]:border-site-light-080",
  "[&_img]:rounded-[14px] [&_img]:border [&_img]:border-site-light-080",
  "[&>:first-child]:mt-0 [&>:last-child]:mb-0"
);
