import { ChevronRight } from "@compozy/ui/icons";
import type { ReactNode } from "react";

import { Icon, StateGlyph, cn } from "@compozy/ui";

import { workspaceWorkingLabel } from "@/systems/workspace";

/**
 * A group or folder header's leading 12 px slot. It holds the fold chevron, which
 * a collapsed header with work running yields at rest to its running signal: the
 * sidebar's one "work is running" column (#1926). Hovering the toggle, or focusing
 * it from the keyboard (it carries the `group/disclosure` class), shows the chevron
 * instead; only one of the two is ever displayed.
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
    <span className="grid size-3 shrink-0 place-items-center">
      <span className="hidden group-hover/disclosure:grid group-focus-visible/disclosure:grid">
        {chevron}
      </span>
      <span className="grid group-hover/disclosure:hidden group-focus-visible/disclosure:hidden">
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
