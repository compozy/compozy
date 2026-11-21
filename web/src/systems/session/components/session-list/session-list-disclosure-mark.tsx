import { ChevronRight } from "@compozy/ui/icons";
import type { ReactNode } from "react";

import { Icon, StateGlyph, cn } from "@compozy/ui";

import { workspaceWorkingLabel } from "@/systems/workspace";

/**
 * A group or folder header's leading 12 px slot. It holds the fold chevron, which
 * a collapsed header with work running yields at rest to its running signal: the
 * sidebar's one "work is running" column (#1926). Hovering the toggle, or focusing
 * it from the keyboard (it carries the `group/disclosure` class), shows the chevron
 * instead; the glyph only fades, so its name stays announced.
 */
export function SessionListDisclosureMark({
  open,
  signal,
}: {
  open: boolean;
  /** Drawn only while the header is collapsed; an open header shows its rows instead. */
  signal?: ReactNode;
}) {
  const chevron = (
    <Icon
      as={ChevronRight}
      size="sm"
      aria-hidden="true"
      className={cn("text-faint transition-transform", open && "rotate-90")}
    />
  );
  if (open || signal === undefined || signal === null) {
    return <span className="grid size-3 shrink-0 place-items-center">{chevron}</span>;
  }
  return (
    // Both layers share one cell; opacity, not display, swaps them so the glyph's
    // "N sessions working" name stays in the accessibility tree while the chevron shows.
    <span className="grid size-3 shrink-0 place-items-center *:col-start-1 *:row-start-1">
      <span className="grid opacity-0 transition-opacity duration-fast group-hover/disclosure:opacity-100 group-focus-visible/disclosure:opacity-100 motion-reduce:transition-none">
        {chevron}
      </span>
      <span className="grid transition-opacity duration-fast group-hover/disclosure:opacity-0 group-focus-visible/disclosure:opacity-0 motion-reduce:transition-none">
        {signal}
      </span>
    </span>
  );
}

/** The leading running glyph for `count` sessions working; render it only while one runs. */
export function SessionListRunningGlyph({ count, slot }: { count: number; slot: string }) {
  return (
    <span data-slot={slot} className="grid place-items-center">
      <StateGlyph size="sm" state="running" label={workspaceWorkingLabel(count)} />
    </span>
  );
}
