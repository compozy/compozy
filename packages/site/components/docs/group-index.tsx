/** The docs home's section map: every section in reading order, numbered, with its page count. */
import { cn } from "@compozy/ui";
import { ArrowRight } from "@compozy/ui/icons";
import Link from "next/link";
import type { ReactNode } from "react";

import { FOCUS, LINE } from "@/components/docs/docs-classes";
import { SectionMapRail } from "@/components/docs/section-map-rail";
import { getDocsShell } from "@/lib/docs-shell";

export function GroupIndex({ children }: { children: ReactNode }) {
  return <SectionMapRail>{children}</SectionMapRail>;
}

export function GroupRow({ name, children }: { name: string; children: ReactNode }) {
  const tab = getDocsShell().tabs.find(candidate => candidate.label === name);
  return (
    <li
      className={cn(
        "relative my-0! grid gap-x-8 gap-y-2 border-b py-6 pl-12 last:border-b-0 md:grid-cols-[230px_minmax(0,1fr)_auto]",
        LINE
      )}
    >
      <span
        aria-hidden
        className="docs-step-number absolute top-6 left-0 inline-flex size-7 items-center justify-center rounded-full border border-site-light-200 bg-[#0a0a0a] font-mono text-[11.5px] text-brand-strong tabular-nums"
      />
      <Link
        href={tab?.href ?? "/docs"}
        className={cn(
          "group inline-flex items-center gap-2 self-start rounded-[6px] text-left font-display text-[22px]/[1.25] tracking-[-0.02em] text-fg",
          FOCUS
        )}
      >
        {name}
        <ArrowRight
          className="size-4 text-subtle transition-transform group-hover:translate-x-1 group-hover:text-fg"
          aria-hidden
        />
      </Link>
      <div className="text-[15px] leading-[1.65] text-fg/80 [&_p]:my-0">{children}</div>
      <span className="self-start pt-1 font-mono text-[12.5px] text-subtle tabular-nums md:text-right">
        {tab?.count ?? 0} pages
      </span>
    </li>
  );
}
