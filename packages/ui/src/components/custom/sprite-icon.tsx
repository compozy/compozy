import * as React from "react";

import { ICON_STROKE_WIDTH } from "../../lib/icon-stroke";
import { cn } from "../../lib/utils";

export interface SpriteIconProps extends React.ComponentProps<"svg"> {
  /** URL of an SVG sprite whose `<symbol>` ids are icon names. */
  spriteUrl: string;
  name: string;
}

/**
 * Renders one icon out of an external SVG sprite; inherits `currentColor`. The
 * sprite is outside `LucideProvider`, so it takes the shared lucide stroke
 * explicitly to draw the same line as every other icon.
 */
export function SpriteIcon({
  spriteUrl,
  name,
  className,
  strokeWidth = ICON_STROKE_WIDTH,
  ...props
}: SpriteIconProps) {
  return (
    <svg
      data-slot="sprite-icon"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={strokeWidth}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      className={cn("size-4 shrink-0", className)}
      {...props}
    >
      <use href={`${spriteUrl}#${name}`} />
    </svg>
  );
}
