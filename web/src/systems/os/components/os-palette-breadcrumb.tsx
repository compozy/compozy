import { CornerUpLeft } from "lucide-react";

import { Icon } from "@compozy/ui";

import { cn } from "@/lib/utils";

import { paletteViewLeadClass } from "../lib/palette-view-inset";
import type { PaletteBreadcrumb } from "../lib/palette-view-stack";

export interface OsPaletteBreadcrumbProps {
  breadcrumb: PaletteBreadcrumb;
}

/**
 * Where the operator is inside the palette.
 *
 * The leaf and its parent always survive; anything above them collapses into a
 * single `…`, so the path stays one readable line at any depth instead of
 * pushing the query field around.
 */
export function OsPaletteBreadcrumb({ breadcrumb }: OsPaletteBreadcrumbProps) {
  const last = breadcrumb.visible.length - 1;
  return (
    <nav
      aria-label="Palette path"
      data-testid="os-palette-breadcrumb"
      className={cn(
        "flex items-center gap-1.5 pt-2 pb-1.5 text-micro text-muted",
        paletteViewLeadClass
      )}
    >
      <Icon as={CornerUpLeft} size="xs" className="shrink-0 text-subtle" aria-hidden="true" />
      {breadcrumb.truncated ? (
        <>
          <span data-slot="os-palette-crumb-more" title="Earlier levels">
            …
          </span>
          <span aria-hidden="true" className="text-faint">
            /
          </span>
        </>
      ) : null}
      {breadcrumb.visible.map((label, index) => (
        <span key={`${index}:${label}`} className="flex items-center gap-1.5">
          <span
            data-slot="os-palette-crumb"
            className={cn("truncate", index === last && "text-fg-strong")}
          >
            {label}
          </span>
          {index === last ? null : (
            <span aria-hidden="true" className="text-faint">
              /
            </span>
          )}
        </span>
      ))}
    </nav>
  );
}
