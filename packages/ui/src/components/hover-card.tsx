"use client";

import { PreviewCard as PreviewCardPrimitive } from "@base-ui/react/preview-card";
import { useReducedMotionConfig } from "motion/react";

import { cn } from "../lib/utils";

/** Hover dwell before the card opens, in ms. Focus opens it at once. */
const HOVER_CARD_OPEN_DELAY_MS = 200;
/** Grace after the pointer leaves, so it can travel into the card, in ms. */
const HOVER_CARD_CLOSE_DELAY_MS = 120;

function HoverCard({ ...props }: PreviewCardPrimitive.Root.Props) {
  return <PreviewCardPrimitive.Root data-slot="hover-card" {...props} />;
}

/**
 * The element that opens the card: hover after `delay`, or keyboard focus.
 * Renders an `<a>` by default; pass `render` to make a button or row the trigger.
 */
function HoverCardTrigger({
  delay = HOVER_CARD_OPEN_DELAY_MS,
  closeDelay = HOVER_CARD_CLOSE_DELAY_MS,
  ...props
}: PreviewCardPrimitive.Trigger.Props) {
  return (
    <PreviewCardPrimitive.Trigger
      data-slot="hover-card-trigger"
      delay={delay}
      closeDelay={closeDelay}
      {...props}
    />
  );
}

type HoverCardContentProps = PreviewCardPrimitive.Popup.Props &
  Pick<
    PreviewCardPrimitive.Positioner.Props,
    "align" | "alignOffset" | "side" | "sideOffset" | "collisionPadding"
  >;

/**
 * Read-only preview on the popover surface. Use it for rich, non-interactive
 * detail about the trigger (metadata, previews); `Tooltip` stays text-only and
 * `Popover` stays click-owned. Closes on pointer leave, blur and Escape.
 */
function HoverCardContent({
  className,
  align = "start",
  alignOffset = 0,
  side = "bottom",
  sideOffset = 6,
  collisionPadding = 8,
  ...props
}: HoverCardContentProps) {
  const reduced = useReducedMotionConfig();
  return (
    <PreviewCardPrimitive.Portal>
      <PreviewCardPrimitive.Positioner
        align={align}
        alignOffset={alignOffset}
        side={side}
        sideOffset={sideOffset}
        collisionPadding={collisionPadding}
        className="isolate z-50"
      >
        <PreviewCardPrimitive.Popup
          data-slot="hover-card-content"
          className={cn(
            "z-50 w-75 max-w-(--available-width) origin-(--transform-origin) rounded-md border border-line-strong bg-elevated px-3 py-2.5 text-small-body text-fg shadow-overlay outline-hidden",
            reduced
              ? "transition-none"
              : "transition-[opacity,translate] duration-fast ease-out data-ending-style:-translate-y-0.5 data-ending-style:opacity-0 data-starting-style:-translate-y-0.5 data-starting-style:opacity-0",
            className
          )}
          {...props}
        />
      </PreviewCardPrimitive.Positioner>
    </PreviewCardPrimitive.Portal>
  );
}

export { HoverCard, HoverCardContent, HoverCardTrigger };
export type { HoverCardContentProps };
