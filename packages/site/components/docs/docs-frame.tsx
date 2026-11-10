"use client";

/**
 * The docs page frame the layout keeps mounted: the sidebar rail beside the page and its copy in
 * the drawer. Living in the layout, the rail keeps its scroll position and the folders the reader
 * opened or closed across client navigations; only the current section's tree changes.
 */
import { cn } from "@compozy/ui";
import { usePathname } from "next/navigation";
import { useState, type ReactNode } from "react";

import { DocsDrawer } from "@/components/docs/docs-chrome";
import { BELOW_HEADER, LINE } from "@/components/docs/docs-classes";
import { DocsSidebar, type DocsSidebarGroup } from "@/components/docs/docs-sidebar";
import { currentSectionId, foldersHolding, normalizeDocsUrl } from "@/lib/docs-navigation";

export interface DocsFrameSection extends DocsSidebarGroup {
  /** Top-level path segments under `/docs/` the section owns. */
  heads: string[];
}

export function DocsFrame({
  sections,
  railless,
  children,
}: {
  sections: readonly DocsFrameSection[];
  /** Pages that render without the rail: the docs home and pages outside the page tree. */
  railless: readonly string[];
  children: ReactNode;
}) {
  const url = normalizeDocsUrl(usePathname());
  const [folders, setFolders] = useState<{ url: string; toggled: Record<string, boolean> }>({
    url,
    toggled: {},
  });
  // Landing on a page inside a folder the reader closed opens it again, with its ancestors, so the
  // current row shows; folders toggled anywhere else keep their state.
  if (folders.url !== url) {
    const reopened = new Set(
      foldersHolding(
        sections.flatMap(section => section.nodes),
        url
      )
    );
    const toggled = Object.fromEntries(
      Object.entries(folders.toggled).filter(([key]) => !reopened.has(key))
    );
    setFolders({ url, toggled });
  }
  const sectionId = currentSectionId(sections, url);
  // The sidebar shows the page's section; pages outside every section see all of them.
  const groups = sectionId ? sections.filter(section => section.id === sectionId) : sections;
  const sidebar = (
    <DocsSidebar
      groups={groups}
      activeUrl={url}
      toggled={folders.toggled}
      onToggle={(key, open) =>
        setFolders(current => ({ ...current, toggled: { ...current.toggled, [key]: open } }))
      }
    />
  );
  const drawer = <DocsDrawer>{sidebar}</DocsDrawer>;
  if (railless.includes(url)) {
    return (
      <>
        {children}
        {drawer}
      </>
    );
  }
  return (
    <div className="mx-auto grid max-w-[1600px] lg:grid-cols-[296px_minmax(0,1fr)]">
      <aside
        className={cn(
          "docs-rail sticky hidden h-[calc(100vh-104px)] overflow-y-auto border-r px-4 pt-8 pb-12 lg:block",
          BELOW_HEADER,
          LINE
        )}
      >
        {sidebar}
      </aside>
      {children}
      {drawer}
    </div>
  );
}
