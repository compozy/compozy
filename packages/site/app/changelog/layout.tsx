import type { ReactNode } from "react";

import { SiteShell } from "@/components/site/site-shell";

export default function ChangelogLayout({ children }: { children: ReactNode }) {
  return (
    <SiteShell>
      {/* Pre-redesign body: offset below the floating header until this surface's slice replaces it. */}
      <div className="site-home flex flex-1 flex-col bg-canvas pt-[76px]">{children}</div>
    </SiteShell>
  );
}
