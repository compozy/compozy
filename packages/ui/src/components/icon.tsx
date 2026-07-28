"use client";

import type { LucideIcon, LucideProps } from "lucide-react";

import { cn } from "../lib/utils";

export type IconSize = "xs" | "sm" | "default" | "lg";

const SIZE_PX: Record<IconSize, number> = {
  xs: 11,
  sm: 12,
  default: 14,
  lg: 16,
};

export interface IconProps extends Omit<LucideProps, "size"> {
  /** Lucide icon component to render. */
  as: LucideIcon;
  /** Size step — `xs` 11 px, `sm` 12 px, `default` 14 px, `lg` 16 px. */
  size?: IconSize;
}

/**
 * Thin helper that enforces the runtime icon contract: 1.75 stroke-width by default,
 * 2 at the 11 px xs floor (per). Callers may pass `strokeWidth` to
 * override for one-off needs.
 */
function Icon({ as: As, size = "default", className, strokeWidth, ref, ...rest }: IconProps) {
  const px = SIZE_PX[size];
  const stroke = strokeWidth ?? (size === "xs" ? 2 : 1.75);
  return (
    <As
      ref={ref}
      width={px}
      height={px}
      strokeWidth={stroke}
      data-icon-size={size}
      className={cn("shrink-0", className)}
      {...rest}
    />
  );
}

export { Icon };
