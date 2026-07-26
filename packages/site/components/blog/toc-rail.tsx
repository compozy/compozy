"use client";

import { cn, Eyebrow } from "@agh/ui";
import Link from "next/link";
import { useEffect, useState } from "react";
import type { TocItem } from "./toc-utils";

export type { TocItem } from "./toc-utils";

export interface TocRailProps {
  items: TocItem[];
}

export function TocRail({ items }: TocRailProps) {
  const [activeId, setActiveId] = useState<string | undefined>(() =>
    items[0]?.url.replace(/^#/, "")
  );

  useEffect(() => {
    if (items.length === 0) return;
    if (typeof IntersectionObserver === "undefined") return;
    const ids = items.map(item => item.url.replace(/^#/, ""));
    const observer = new IntersectionObserver(
      entries => {
        let visible: IntersectionObserverEntry | undefined;
        for (const entry of entries) {
          if (!entry.isIntersecting) continue;
          if (!visible || entry.boundingClientRect.top < visible.boundingClientRect.top) {
            visible = entry;
          }
        }
        if (visible?.target.id) {
          setActiveId(visible.target.id);
        }
      },
      { rootMargin: "-30% 0px -55% 0px", threshold: [0, 1] }
    );
    ids.forEach(id => {
      const el = document.getElementById(id);
      if (el) observer.observe(el);
    });
    return () => observer.disconnect();
  }, [items]);

  if (items.length === 0) return null;

  return (
    <aside aria-label="Blog table of contents" className="sticky top-20 self-start">
      <Eyebrow className="text-muted tracking-badge!">On this page</Eyebrow>
      <ul className="mt-4 flex flex-col gap-2.5">
        {items.map(item => {
          const id = item.url.replace(/^#/, "");
          const isActive = id === activeId;
          return (
            <li key={item.url}>
              <Link
                href={item.url}
                aria-current={isActive ? "location" : undefined}
                className={cn(
                  "block text-small-body leading-5 transition-colors",
                  isActive ? "text-accent" : "text-muted hover:text-fg",
                  item.depth >= 3 && "pl-3"
                )}
              >
                {item.title}
              </Link>
            </li>
          );
        })}
      </ul>
    </aside>
  );
}
