// Previous / next chapter cards; the next card carries the chapter's cover visual.
import { cn } from "@compozy/ui/utils";
import { Link } from "@tanstack/react-router";

import { Visual } from "../art/visual";
import { hrefOf, pages, type Chapter } from "../chapters";

const SCRIM = {
  dark: "bg-[linear-gradient(90deg,color-mix(in_oklab,var(--color-rail)_90%,transparent),transparent_70%)]",
  light: "bg-[linear-gradient(90deg,var(--color-fg),transparent_70%)]",
};

export function ChapterNeighbours({ page }: { page: Chapter }) {
  const i = pages.indexOf(page);
  const prev = pages[i - 1];
  const next = pages[i + 1];
  return (
    <nav aria-label="Neighbouring chapters" className="border-t border-line bg-rail">
      <div className="wrap grid gap-4 py-10 lg:grid-cols-[minmax(0,1fr)_minmax(0,2fr)]">
        {prev ? (
          <Link
            to={hrefOf(prev)}
            data-neighbour="previous"
            className="group grid content-end gap-1 rounded-card border border-line p-6 no-underline hover:border-line-strong"
          >
            <span className="mono-label tracking-[0.1em] text-muted-foreground">
              ← previous · {prev.n}
            </span>
            <span className="font-display text-[24px] tracking-[-0.02em] group-hover:text-(--color-brand-cyan)">
              {prev.title}
            </span>
            <span className="text-sm text-muted-foreground">{prev.summary}</span>
          </Link>
        ) : (
          <span />
        )}
        {next && (
          <Link
            to={hrefOf(next)}
            data-neighbour="next"
            className={cn(
              "group relative isolate grid min-h-[240px] content-end gap-1 overflow-hidden rounded-card p-6 no-underline",
              next.cover.theme === "light" ? "band-paper" : "band-ink2"
            )}
          >
            <Visual
              name={next.cover.visual}
              theme={next.cover.theme}
              x={next.cover.x}
              y={next.cover.y}
              sizes="(min-width: 1024px) 66vw, 100vw"
              className="-z-10 transition-transform duration-700 ease-out group-hover:scale-[1.03]"
            />
            <div className={cn("absolute inset-0 -z-10", SCRIM[next.cover.theme ?? "dark"])} />
            <span className="mono-label text-(--b-muted)">next · {next.n} →</span>
            <span className="font-display text-[clamp(28px,3vw,40px)] tracking-[-0.025em] text-(--b-fg)">
              {next.title}
            </span>
            <span className="max-w-[46ch] text-sm text-(--b-muted)">{next.summary}</span>
          </Link>
        )}
      </div>
    </nav>
  );
}
