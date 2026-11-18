// Chapter cover: full-bleed visual with number, title, summary and spec strip.
import { cn } from "@compozy/ui/utils";
import type { ReactNode } from "react";

import { Visual } from "../art/visual";
import { pages, VERSION, type Chapter } from "../chapters";
import { StatusTag } from "./status-tag";

const SCRIM = {
  dark: "bg-[linear-gradient(90deg,color-mix(in_oklab,var(--color-rail)_96%,transparent)_0%,color-mix(in_oklab,var(--color-rail)_78%,transparent)_36%,transparent_72%)]",
  light:
    "bg-[linear-gradient(90deg,var(--color-fg)_0%,color-mix(in_oklab,var(--color-fg)_92%,transparent)_34%,transparent_70%)]",
};

interface PageCoverProps {
  page: Chapter;
  title: ReactNode;
  children?: ReactNode;
  front?: boolean;
}

export function PageCover({ page, title, children, front = false }: PageCoverProps) {
  const theme = page.cover.theme ?? "dark";
  return (
    <header
      data-cover={page.cover.visual}
      className={cn(
        "relative isolate overflow-hidden",
        theme === "light" ? "band-paper" : "band-ink"
      )}
    >
      <Visual
        name={page.cover.visual}
        theme={page.cover.theme}
        x={page.cover.x}
        y={page.cover.y}
        className="visual-drift -z-10"
      />
      <div className={cn("absolute inset-0 -z-10", SCRIM[theme])} />
      <div
        className={cn(
          "wrap grid content-end gap-6 pt-24 pb-14",
          front ? "min-h-[min(92vh,880px)]" : "min-h-[min(78vh,720px)]"
        )}
      >
        <div className="flex flex-wrap items-center gap-3 mono-label tracking-[0.12em] text-(--b-muted)">
          <span className="text-(--b-accent)">{page.n}</span>
          <span>/ {pages.length}</span>
          <StatusTag status={page.status} />
        </div>
        <h1 className="m-0 max-w-[16ch] text-display text-(--b-fg)">{title}</h1>
        {children && <div className="max-w-[52ch] text-lead text-(--b-muted)">{children}</div>}
        <dl className="mt-6 flex flex-wrap gap-x-10 gap-y-2 mono-label tracking-[0.1em] text-(--b-muted) [&_dd]:ml-2 [&_dd]:inline [&_dd]:text-(--b-fg) [&_dt]:inline">
          <div>
            <dt>Chapter</dt>
            <dd>{page.title}</dd>
          </div>
          <div>
            <dt>Sections</dt>
            <dd>{page.sections.length}</dd>
          </div>
          <div>
            <dt>Cover</dt>
            <dd>{page.cover.visual}</dd>
          </div>
          <div>
            <dt>Edition</dt>
            <dd>{VERSION}</dd>
          </div>
        </dl>
      </div>
    </header>
  );
}
