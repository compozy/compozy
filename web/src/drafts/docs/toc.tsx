/** "On this page": the section list beside the article, with the brand beam filling as you read. */
import { cn } from "@compozy/ui";
import { m, useReducedMotion, useScroll } from "motion/react";
import type { RefObject } from "react";

import { useActiveHeading } from "./hooks/use-active-heading";
import type { TocItem } from "./mdx";
import { BEAM, BELOW_HEADER } from "./text";

export function DocToc({
  toc,
  body,
}: {
  toc: readonly TocItem[];
  body: RefObject<HTMLElement | null>;
}) {
  const ids = toc.map(item => item.id);
  const active = useActiveHeading(ids);
  const reduced = useReducedMotion();
  const { scrollYProgress } = useScroll({ target: body, offset: ["start 35%", "end 65%"] });
  if (toc.length === 0) return <div className="hidden xl:block" />;
  return (
    <nav
      aria-label="On this page"
      className={cn(
        "docs-rail hidden max-h-[calc(100vh-136px)] overflow-y-auto pt-10 pb-10 xl:sticky xl:block xl:self-start",
        BELOW_HEADER
      )}
    >
      <p className="m-0 mb-4 text-[13px] text-subtle">On this page</p>
      <div className="relative">
        <span aria-hidden className="absolute inset-y-0 left-0 w-px bg-[color:var(--light-120)]">
          <m.span
            style={{ scaleY: reduced ? 1 : scrollYProgress }}
            className={cn("absolute inset-y-0 -left-px w-0.5 origin-top", BEAM)}
          />
        </span>
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
                {item.text}
              </a>
            </li>
          ))}
        </ol>
      </div>
    </nav>
  );
}
