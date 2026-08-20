import * as React from "react";

import { cn } from "../../lib/utils";

export type EyebrowVariant = "default" | "caps";

export type EyebrowProps = Omit<React.ComponentProps<"span">, "children"> & {
  children: React.ReactNode;
  /**
   * `default` is sentence case (12/510/-0.005em). `caps` is the opt-in
   * uppercase kicker for the few labels that are true typographic kickers —
   * it renders the fixed `eyebrow-caps` rendition (11/600/+0.06em), a size
   * step down with positive tracking because uppercase reads optically
   * larger. Both renditions live in tokens-runtime.css, so there is still
   * one eyebrow contract (L-022) with no free parameters.
   */
  variant?: EyebrowVariant;
};

function Eyebrow({ className, children, variant = "default", ...props }: EyebrowProps) {
  return (
    <span
      data-slot="eyebrow"
      data-variant={variant === "caps" ? "caps" : undefined}
      className={cn(variant === "caps" ? "eyebrow-caps" : "eyebrow", className)}
      {...props}
    >
      {children}
    </span>
  );
}

export { Eyebrow };
