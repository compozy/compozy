import { ChevronRight } from "@compozy/ui/icons";

import { Icon, StateGlyph, cn } from "@compozy/ui";

import { sessionListLeadingSignalLabel } from "../../lib/session-list-groups";

/** What a collapsed header's rows still say in its leading column (#1926). */
export interface SessionListLeadingSignal {
  needsYou: number;
  working: number;
  /** `data-slot` prefix; the glyph's slot ends in `-needs-you` or `-working`. */
  slot: "session-group" | "session-folder";
}

/**
 * A group or folder header's leading 12 px slot: the sidebar's one state column. It
 * holds the fold chevron, which a collapsed header yields at rest to one aggregate
 * glyph with the session rows' precedence, needs you over running (#1926). Hovering
 * the toggle, or focusing it from the keyboard (it carries the `group/disclosure`
 * class), shows the chevron instead; the glyph only fades, so its name stays announced.
 * A device without hover never sees that swap, so there the chevron keeps its cell and
 * the glyph sits in a second cell beside it.
 */
export function SessionListDisclosureMark({
  open,
  signal,
}: {
  open: boolean;
  /** Drawn only while the header is collapsed; an open header shows its rows instead. */
  signal?: SessionListLeadingSignal;
}) {
  const chevron = (
    <Icon
      as={ChevronRight}
      size="sm"
      aria-hidden="true"
      className={cn("text-faint transition-transform", open && "rotate-90")}
    />
  );
  const label = open || !signal ? null : sessionListLeadingSignalLabel(signal);
  if (!signal || label === null) {
    return <span className="grid size-3 shrink-0 place-items-center">{chevron}</span>;
  }
  const needsYou = signal.needsYou > 0;
  return (
    // Both layers share one cell; opacity, not display, swaps them so the glyph's
    // name stays in the accessibility tree while the chevron shows. Without hover the
    // two cells sit side by side, both shown.
    <span className="grid size-3 shrink-0 place-items-center *:col-start-1 *:row-start-1 [@media(hover:none)]:flex [@media(hover:none)]:w-auto [@media(hover:none)]:gap-1">
      <span className="grid size-3 place-items-center opacity-0 transition-opacity duration-fast group-hover/disclosure:opacity-100 group-focus-visible/disclosure:opacity-100 motion-reduce:transition-none [@media(hover:none)]:opacity-100!">
        {chevron}
      </span>
      <span
        data-slot={`${signal.slot}-${needsYou ? "needs-you" : "working"}`}
        className="grid size-3 place-items-center transition-opacity duration-fast group-hover/disclosure:opacity-0 group-focus-visible/disclosure:opacity-0 motion-reduce:transition-none [@media(hover:none)]:opacity-100!"
      >
        <StateGlyph size="sm" state={needsYou ? "attention" : "running"} label={label} />
      </span>
    </span>
  );
}
