"use client";

import { cn } from "@compozy/ui";
import { ChevronRight } from "@compozy/ui/icons";
import { useState } from "react";

import { LINE } from "@/components/docs/docs-classes";
import { useActiveHeading } from "@/components/site/hooks/use-active-heading";
import { BEAM, FOCUS } from "@/components/site/layout";

export interface LegalTocItem {
  readonly id: string;
  readonly title: string;
}

function LegalTocList({
  items,
  activeId,
  onSelect,
}: {
  items: readonly LegalTocItem[];
  activeId: string | null;
  onSelect?: () => void;
}) {
  return (
    <ol className="m-0 list-none p-0">
      {items.map(item => {
        const current = activeId === item.id;
        return (
          <li key={item.id}>
            <a
              href={`#${item.id}`}
              aria-current={current ? "location" : undefined}
              onClick={onSelect}
              className={cn(
                "block rounded-[6px] py-1.5 pl-4 text-[13.5px]/[1.4] transition-colors",
                FOCUS,
                current ? "text-fg" : "text-muted-foreground hover:text-fg"
              )}
            >
              {item.title}
            </a>
          </li>
        );
      })}
    </ol>
  );
}

export function LegalMobileToc({
  items,
  defaultOpen = false,
}: {
  items: readonly LegalTocItem[];
  defaultOpen?: boolean;
}) {
  const [open, setOpen] = useState(defaultOpen);
  const activeId = useActiveHeading(items.map(item => item.id));

  return (
    <div className={cn("mt-7 rounded-[14px] border bg-[#0e0e0e] xl:hidden", LINE)}>
      <button
        type="button"
        aria-expanded={open}
        onClick={() => setOpen(value => !value)}
        className={cn(
          "flex h-11 w-full cursor-pointer items-center gap-2 rounded-[14px] px-4 text-[13.5px] text-muted-foreground transition-colors hover:text-fg",
          FOCUS
        )}
      >
        <ChevronRight
          aria-hidden
          className={cn("size-3.5 text-subtle transition-transform", open && "rotate-90")}
        />
        <span>On this page</span>
        <span className="ml-auto font-mono text-[11.5px] text-subtle tabular-nums">
          {items.length}
        </span>
      </button>
      {open ? (
        <div className="relative mx-4 mb-4 border-l border-site-light-120">
          <LegalTocList items={items} activeId={activeId} onSelect={() => setOpen(false)} />
        </div>
      ) : null}
    </div>
  );
}

export function LegalDesktopToc({ items }: { items: readonly LegalTocItem[] }) {
  const activeId = useActiveHeading(items.map(item => item.id));

  return (
    <nav
      aria-label="On this page"
      className="docs-rail hidden max-h-[calc(100vh-152px)] overflow-y-auto pt-2 pb-10 xl:sticky xl:top-28 xl:block xl:self-start"
    >
      <p className="m-0 mb-4 text-[13px] text-subtle">On this page</p>
      <div className="relative">
        <span aria-hidden className="absolute inset-y-0 left-0 w-px bg-site-light-120">
          <span aria-hidden className={cn("absolute inset-y-0 -left-px w-0.5", BEAM)} />
        </span>
        <LegalTocList items={items} activeId={activeId} />
      </div>
    </nav>
  );
}
