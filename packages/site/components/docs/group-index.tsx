import { Eyebrow } from "@compozy/ui";
import type { ReactNode } from "react";

/**
 * The eight sidebar groups explained in reading order. The group name reuses the sidebar's own
 * section-label tone (`components/site/sidebar-section-label.tsx`) so the page and the rail name
 * the same thing the same way.
 */
export function GroupIndex({ children }: { children: ReactNode }) {
  return <div className="not-prose my-8 border-t border-line">{children}</div>;
}

export function GroupRow({ name, children }: { name: string; children: ReactNode }) {
  return (
    <div className="grid gap-2 border-b border-line py-4 md:grid-cols-[190px_minmax(0,1fr)] md:gap-5">
      <Eyebrow className="pt-[3px] font-mono text-badge font-medium tracking-[0.09em] text-[color-mix(in_srgb,var(--color-accent)_65%,var(--color-muted))]">
        {name}
      </Eyebrow>
      {/* Links inside inherit `.site-doc-body :is(p, …) a` from global.css — the same underline
          treatment every other doc paragraph uses. */}
      <p className="text-sm leading-[1.65] text-muted">{children}</p>
    </div>
  );
}
