"use client";

import * as React from "react";

import { cn } from "../../lib/utils";

export type StatusDotTone = "warning" | "danger" | "accent" | "faint";
export type StatusDotVariant = "solid" | "ring";
export type StatusDotSize = "default" | "sm";

export interface StatusDotProps extends Omit<React.ComponentProps<"span">, "children"> {
  /** Color family applied to the dot (per inbox vocabulary). */
  tone: StatusDotTone;
  /** `solid` fills the dot; `ring` paints a 1 px outline only. */
  variant?: StatusDotVariant;
  /** `default` = 7 px; `sm` = 6 px. */
  size?: StatusDotSize;
  /**
   * Accessible label. Defaults to the underlying `data-tone` + `data-variant`
   * combination but consumers should supply a domain-specific label
   * (e.g., "Needs review", "Mentions").
   */
  label?: string;
}

const TONE_TEXT_COLOR: Record<StatusDotTone, string> = {
  warning: "text-warning",
  danger: "text-danger",
  accent: "text-accent",
  faint: "text-faint",
};

const SIZE_PX: Record<StatusDotSize, string> = {
  default: "size-status-dot",
  sm: "size-status-dot-sm",
};

function StatusDot({
  tone,
  variant = "solid",
  size = "default",
  label,
  className,
  ...props
}: StatusDotProps) {
  const ariaLabel = label;
  return (
    <span
      data-slot="status-dot"
      data-tone={tone}
      data-variant={variant}
      data-size={size}
      role="img"
      aria-label={ariaLabel}
      aria-hidden={ariaLabel ? undefined : "true"}
      className={cn(
        "inline-block rounded-full",
        SIZE_PX[size],
        TONE_TEXT_COLOR[tone],
        variant === "solid" ? "bg-current" : "border border-current bg-transparent",
        className
      )}
      {...props}
    />
  );
}

export { StatusDot };
