import * as React from "react";

import { cn } from "../../lib/utils";

export type EyebrowVariant = "default" | "caps";

export type EyebrowProps = Omit<React.ComponentProps<"span">, "children"> & {
  children: React.ReactNode;
  /**
   * `default` is sentence case. `caps` is the opt-in uppercase kicker for the
   * few labels that are true typographic kickers; it rides the same size and
   * tracking tokens, so there is still one eyebrow contract (L-022).
   */
  variant?: EyebrowVariant;
};

function Eyebrow({ className, children, variant = "default", ...props }: EyebrowProps) {
  return (
    <span
      data-slot="eyebrow"
      data-variant={variant === "caps" ? "caps" : undefined}
      className={cn("eyebrow", variant === "caps" && "uppercase", className)}
      {...props}
    >
      {children}
    </span>
  );
}

export { Eyebrow };
