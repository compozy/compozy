import type { ReactNode } from "react";

import { DocsChromeProvider } from "@/components/docs/docs-chrome";
import { DocsFrame } from "@/components/docs/docs-frame";
import { DocsHeader } from "@/components/docs/docs-header";
import { DocsSearch } from "@/components/docs/docs-search";
import { SiteMotionProvider } from "@/components/site/motion-provider";
import { SiteFooter } from "@/components/site/site-footer";
import { getDocsShell } from "@/lib/docs-shell";
import { normalizeDocsUrl } from "@/lib/docs-navigation";
import { docsSource } from "@/lib/source";

/**
 * The docs shell: header with section tabs, the sidebar frame around the page, the search palette
 * and the site footer. The frame lives here so the rail persists across navigations.
 */
export default function DocsRouteLayout({ children }: { children: ReactNode }) {
  const shell = getDocsShell();
  const start = shell.sections[0];
  const suggested = shell.pages
    .filter(page => page.section?.id === start?.id)
    .slice(0, 6)
    .map(page => ({ url: page.url, title: page.title, description: page.description }));
  const icons: Record<string, ReactNode> = {};
  for (const section of shell.sections) {
    for (const node of section.nodes) {
      const head = (node.url ?? node.children?.find(child => child.url)?.url)
        ?.replace(/^\/docs\/?/, "")
        .split("/")[0];
      if (head && node.icon) icons[head] = node.icon;
    }
  }
  const sections = shell.sections.map(({ id, label, count, heads, nodes }) => ({
    id,
    label,
    count,
    heads,
    nodes,
  }));
  // The docs home, and any page the tree does not place, render without the rail.
  const railless = ["/docs"];
  for (const page of docsSource.getPages()) {
    const url = normalizeDocsUrl(page.url);
    if (!shell.byUrl.has(url)) railless.push(url);
  }
  return (
    <SiteMotionProvider>
      <DocsChromeProvider>
        <div className="site-docs flex flex-1 flex-col bg-[#0a0a0a] font-sans text-fg antialiased">
          <DocsHeader tabs={shell.tabs} />
          <main id="main-content" className="docs-root min-h-screen flex-1 pt-[104px]">
            <DocsFrame sections={sections} railless={railless}>
              {children}
            </DocsFrame>
          </main>
          <SiteFooter />
          <DocsSearch tabs={shell.tabs} total={shell.total} suggested={suggested} icons={icons} />
        </div>
      </DocsChromeProvider>
    </SiteMotionProvider>
  );
}
