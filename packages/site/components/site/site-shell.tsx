/** The marketing shell: floating header, the page, and the one-line footer on the #0a0a0a surface. */
import type { ReactNode } from "react";

import { SiteMotionProvider } from "@/components/site/motion-provider";
import { SiteFooter } from "@/components/site/site-footer";
import { SiteHeader } from "@/components/site/site-header";

export function SiteShell({ children }: { children: ReactNode }) {
  return (
    <SiteMotionProvider>
      <div className="site-marketing flex flex-1 flex-col bg-[#0a0a0a] font-sans text-fg antialiased">
        <SiteHeader />
        <main id="main-content" className="flex flex-1 flex-col">
          {children}
        </main>
        <SiteFooter />
      </div>
    </SiteMotionProvider>
  );
}
