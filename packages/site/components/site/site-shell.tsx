/** The marketing shell: floating header, the page, and the one-line footer on the #0a0a0a surface. */
import type { ReactNode } from "react";

import { SiteMotionProvider } from "@/components/site/motion-provider";
import { SiteFooter } from "@/components/site/site-footer";
import { SiteHeader } from "@/components/site/site-header";

const NO_SCRIPT_REVEAL =
  "[data-site-reveal]{opacity:1!important;transform:none!important;filter:none!important}";

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
      {/* Scroll reveals start hidden and wait for JavaScript; without it, show them settled. */}
      <noscript>
        <style>{NO_SCRIPT_REVEAL}</style>
      </noscript>
    </SiteMotionProvider>
  );
}
