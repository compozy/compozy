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
    <div className="grid gap-2 border-b border-line py-4 md:grid-cols-[var(--site-doc-index-label-width)_minmax(0,1fr)] md:gap-5">
      <Eyebrow className="pt-0.5 text-subtle">{name}</Eyebrow>
      {/* Links inside inherit `.site-doc-body :is(p, …) a` from docs.css — the same underline
          treatment every other doc paragraph uses. */}
      <div className="text-card-title leading-prose text-muted">{children}</div>
    </div>
  );
}
