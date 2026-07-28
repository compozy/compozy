"use client";

import type { Separator } from "fumadocs-core/page-tree";

export function DocsSidebarSectionLabel({ item }: { item: Separator }) {
  return (
    <p className="mt-5 mb-1 px-2 text-badge font-semibold uppercase tracking-badge text-fd-muted-foreground/70 first:mt-3">
      {item.icon}
      {item.name}
    </p>
  );
}
