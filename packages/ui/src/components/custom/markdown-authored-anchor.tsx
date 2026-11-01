import type * as React from "react";

import { Globe } from "../../icons";
import { cn } from "../../lib/utils";

const EXTERNAL_HREF = /^(?:https?:\/\/|mailto:)/iu;

/**
 * A link in typed chat input: only http(s) and mailto open, in a new tab with
 * no opener or referrer, behind the globe affordance; a long URL truncates and
 * keeps its full address in the tooltip. Any other destination reads as text.
 */
export function MarkdownAuthoredAnchor({
  href,
  children,
  className,
  node: _node,
}: React.ComponentProps<"a"> & { node?: unknown }) {
  if (!href || !EXTERNAL_HREF.test(href)) return <>{children}</>;
  return (
    <a
      href={href}
      title={href}
      target="_blank"
      rel="noopener noreferrer"
      data-slot="markdown-authored-link"
      className={cn(
        "inline-flex max-w-full items-baseline gap-1 align-bottom text-info hover:underline",
        className
      )}
    >
      <Globe aria-hidden="true" className="size-[1em] shrink-0 self-center" />
      <span className="min-w-0 truncate">{children}</span>
    </a>
  );
}
