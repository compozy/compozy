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
  /**
   * The effective chord that opens the selected row's actions. It arrives as a
   * formatted label from the daemon keymap — the footer never spells a chord
   * itself (BR-6) — and is omitted when nothing is selected to act on.
   */
  actionsChord?: string;
  className?: string;
}

/**
 * The palette's keyboard contract, stated where it is used.
 *
 * Nested views add keys the flat palette never had — ⌫ steps back out, and ⇥
 * reaches a view's filters — so the surface says so rather than leaving the
 * operator to discover that backspace does something other than delete. Each
 * hint is rendered only where it is true.
 */
export function OsPaletteFooter({
  enterHint,
  backHint,
  hasFilters,
  actionsChord,
  className,
}: OsPaletteFooterProps) {
  return (
    <div
      data-testid="os-palette-footer"
      className={cn(
        "flex flex-wrap items-center gap-x-5 gap-y-2 border-t border-line px-3.5 py-2.5 text-micro text-subtle",
        className
      )}
    >
      <Hint keys="↑↓">move</Hint>
      <Hint keys="⏎">{enterHint}</Hint>
      {actionsChord === undefined ? null : <Hint keys={actionsChord}>actions</Hint>}
      {hasFilters === true ? <Hint keys="⇥">filters</Hint> : null}
      {backHint === undefined ? null : <Hint keys="⌫">{backHint}</Hint>}
      <Hint keys="esc" className="ml-auto">
        close
      </Hint>
    </div>
  );
}
