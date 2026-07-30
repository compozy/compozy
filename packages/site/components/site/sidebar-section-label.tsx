"use client";

import { Eyebrow } from "@compozy/ui";
import type { Separator } from "fumadocs-core/page-tree";
import { useFolderDepth } from "fumadocs-ui/components/sidebar/base";

function SectionLabel({ item }: { item: Separator }) {
  return (
    <p className="docs-sb-label mt-site-doc-sidebar-section-label-offset mb-1.5 px-2 first:mt-1">
      <Eyebrow className="font-mono text-badge font-medium tracking-site-doc-sidebar-section-label text-site-doc-sidebar-section-label">
        {item.icon}
        {item.name}
      </Eyebrow>
    </p>
  );
}

function GroupLabel({ item }: { item: Separator }) {
  return (
    <p className="docs-sb-group mt-3 mb-0.5 flex items-center gap-site-doc-sidebar-group-gap px-2 first:mt-1.5">
      <Eyebrow className="font-mono text-group-label font-medium tracking-site-doc-sidebar-group-label text-site-doc-sidebar-group-label">
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
