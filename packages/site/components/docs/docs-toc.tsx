"use client";

/** "On this page": the section list beside the article, with the brand beam filling as you read. */
import { cn } from "@compozy/ui";
import { ChevronRight } from "@compozy/ui/icons";
import { m, useReducedMotion, useScroll } from "motion/react";
import { useRef, type ReactNode } from "react";

import { BEAM, BELOW_HEADER, LINE } from "@/components/docs/docs-classes";
import { useActiveHeading } from "@/components/docs/hooks/use-active-heading";

export interface DocsTocItem {
  id: string;
  title: ReactNode;
  depth: number;
}

function TocList({ toc, active }: { toc: readonly DocsTocItem[]; active: string | null }) {
  return (
    <ol className="m-0 list-none p-0">
      {toc.map(item => (
        <li key={item.id}>
          <a
            href={`#${item.id}`}
            aria-current={active === item.id ? "location" : undefined}
            className={cn(
              "block py-1.5 text-[13.5px]/[1.4] transition-colors",
              item.depth === 3 ? "pl-8" : "pl-4",
              active === item.id ? "text-fg" : "text-muted-foreground hover:text-fg"
            )}
          >
            {item.title}
          </a>
        </li>
      ))}
    </ol>
  );
}

/**
 * The reading frame: the article column and, from `xl`, the "On this page" rail. Client-side only
 * because the rail's beam follows the article's scroll.
 */
export function DocsPageFrame({
  toc,
  wide = false,
  children,
}: {
  toc: readonly DocsTocItem[];
  /**
   * Generated API pages keep their two-column operations: the column drops its reading width and
   * the rail, which the operations need the room of.
   */
  wide?: boolean;
  children: ReactNode;
}) {
  const body = useRef<HTMLElement>(null);
  const ids = toc.map(item => item.id);
  const active = useActiveHeading(ids);
  const reduced = useReducedMotion();
  const { scrollYProgress } = useScroll({ target: body, offset: ["start 35%", "end 65%"] });
  return (
    <div
      className={cn(
        "grid min-w-0 gap-x-14 px-5 md:px-10 2xl:gap-x-20",
        !wide && "xl:grid-cols-[minmax(0,1fr)_224px]"
      )}
    >
      <article
        ref={body}
        className={cn("min-w-0 pt-10 pb-24", wide ? "max-w-[1180px]" : "max-w-[800px]")}
      >
        {children}
      </article>
      {wide ? null : toc.length === 0 ? (
        <div className="hidden xl:block" />
      ) : (
        <nav
          aria-label="On this page"
          className={cn(
            "docs-rail hidden max-h-[calc(100vh-136px)] overflow-y-auto pt-10 pb-10 xl:sticky xl:block xl:self-start",
            BELOW_HEADER
          )}
        >
          <p className="m-0 mb-4 text-[13px] text-subtle">On this page</p>
          <div className="relative">
            <span aria-hidden className="absolute inset-y-0 left-0 w-px bg-site-light-120">
              <m.span
                style={{ scaleY: reduced ? 1 : scrollYProgress }}
                className={cn("absolute inset-y-0 -left-px w-0.5 origin-top", BEAM)}
              />
            </span>
            <TocList toc={toc} active={active} />
          </div>
        </nav>
      )}
    </div>
  );
}

/** Below `xl` the rail is hidden: the same list, collapsed under the masthead. */
export function DocsTocDisclosure({ toc }: { toc: readonly DocsTocItem[] }) {
  if (toc.length === 0) return null;
  return (
    <details className={cn("group mt-7 rounded-[14px] border xl:hidden", LINE)}>
      <summary className="flex h-11 cursor-pointer list-none items-center gap-2 px-4 text-[13.5px] text-muted-foreground transition-colors hover:text-fg [&::-webkit-details-marker]:hidden">
        <ChevronRight
          aria-hidden
          className="size-3.5 text-subtle transition-transform group-open:rotate-90"
        />
        On this page
        <span className="ml-auto font-mono text-[11.5px] text-subtle tabular-nums">
          {toc.length}
        </span>
      </summary>
      <div className={cn("relative mx-4 mb-4 border-l", "border-site-light-120")}>
        <TocList toc={toc} active={null} />
      </div>
    </details>
  );
}
