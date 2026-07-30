"use client";

import { Eyebrow } from "@compozy/ui";
import type { Separator } from "fumadocs-core/page-tree";
import { useFolderDepth } from "fumadocs-ui/components/sidebar/base";

function SectionLabel({ item }: { item: Separator }) {
  return (
    <p className="docs-sb-label mt-[26px] mb-1.5 px-2 first:mt-1">
      <Eyebrow className="font-mono text-badge font-medium tracking-[0.09em] text-[color-mix(in_srgb,var(--color-accent)_65%,var(--color-muted))]">
        {item.icon}
        {item.name}
      </Eyebrow>
    </p>
  );
}

function GroupLabel({ item }: { item: Separator }) {
  return (
    <p className="docs-sb-group mt-3 mb-0.5 flex items-center gap-[7px] px-2 first:mt-1.5">
      <Eyebrow className="font-mono text-group-label font-medium tracking-[0.08em] text-[color-mix(in_srgb,var(--color-subtle)_72%,transparent)]">
        {item.icon}
        {item.name}
      </Eyebrow>
      <span aria-hidden className="docs-sb-group-line h-px min-w-0 flex-1 bg-line" />
    </p>
  );
}

export function DocsSidebarSectionLabel({ item }: { item: Separator }) {
  const depth = useFolderDepth();
  if (depth > 0) {
    return <GroupLabel item={item} />;
  }
  return <SectionLabel item={item} />;
}
