import type { ReactNode } from "react";

import { DocsChromeProvider } from "@/components/docs/docs-chrome";
import { DocsHeader } from "@/components/docs/docs-header";
import { DocsSearch } from "@/components/docs/docs-search";
import { SiteMotionProvider } from "@/components/site/motion-provider";
import { SiteFooter } from "@/components/site/site-footer";
import { getDocsShell } from "@/lib/docs-shell";

/** The docs shell: header with section tabs, the page, the search palette and the site footer. */
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
  return (
    <SiteMotionProvider>
      <DocsChromeProvider>
        <div className="site-docs flex flex-1 flex-col bg-[#0a0a0a] font-sans text-fg antialiased">
          <DocsHeader tabs={shell.tabs} />
          <main id="main-content" className="docs-root min-h-screen flex-1 pt-[104px]">
            {children}
          </main>
          <SiteFooter />
          <DocsSearch tabs={shell.tabs} total={shell.total} suggested={suggested} icons={icons} />
        </div>
      </DocsChromeProvider>
    </SiteMotionProvider>
  );
}
