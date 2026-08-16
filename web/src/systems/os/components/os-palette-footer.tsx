import type { ReactNode } from "react";

import { Kbd } from "@compozy/ui";

import { cn } from "@/lib/utils";

interface HintProps {
  keys: string;
  className?: string;
  children: ReactNode;
}

function Hint({ keys, className, children }: HintProps) {
  return (
    <span className={cn("flex items-center gap-1.5", className)}>
      <Kbd>{keys}</Kbd>
      {children}
    </span>
  );
}

export interface OsPaletteFooterProps {
  /** What ⏎ does at this level. */
  enterHint: string;
  /** What ⌫ does on an empty query. Omitted at the root, where it does nothing. */
  backHint?: string;
  /** True when the active view renders filter controls ⇥ can reach. */
  hasFilters?: boolean;
}

/**
 * The palette's keyboard contract, stated where it is used.
 *
 * Nested views add keys the flat palette never had — ⌫ steps back out, and ⇥
 * reaches a view's filters — so the surface says so rather than leaving the
 * operator to discover that backspace does something other than delete. Each
 * hint is rendered only where it is true.
 */
export function OsPaletteFooter({ enterHint, backHint, hasFilters }: OsPaletteFooterProps) {
  return (
    <div
      data-testid="os-palette-footer"
      className="flex items-center gap-3 border-t border-line px-3 py-1.5 text-micro text-subtle"
    >
      <Hint keys="↑↓">move</Hint>
      <Hint keys="⏎">{enterHint}</Hint>
      {hasFilters === true ? <Hint keys="⇥">filters</Hint> : null}
      {backHint === undefined ? null : <Hint keys="⌫">{backHint}</Hint>}
      <Hint keys="esc" className="ml-auto">
        close
      </Hint>
    </div>
  );
}
