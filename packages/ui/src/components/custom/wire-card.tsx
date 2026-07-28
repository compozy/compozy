"use client";

import * as React from "react";

import { cn } from "../../lib/utils";

/**
 * Bordered protocol card , mirrors `.wire-card` (head/body/foot) in
 * `docs/design/web-inspiration/styles/app.css`. Used to embed wire-protocol
 * payloads (recipes, receipts, capability descriptors) inside message
 * threads. Pair with {@link WireCardHead} / {@link WireCardFoot}.
 */
export interface WireCardProps extends React.ComponentProps<"div"> {
  /** Render as a single-line inline strip (used for receipts). */
  inline?: boolean;
}

function WireCard({ inline = false, className, ...props }: WireCardProps) {
  return (
    <div
      {...props}
      data-slot="wire-card"
      data-inline={inline ? "true" : undefined}
      className={cn(
        "bg-canvas-soft",
        inline
          ? "inline-flex items-center gap-2 rounded px-2.5 py-1.5"
          : "max-w-wire-card-max overflow-hidden rounded",
        className
      )}
    />
  );
}

function WireCardHead({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      {...props}
      data-slot="wire-card-head"
      className={cn(
        "eyebrow flex items-center gap-1.5 border-b border-line bg-canvas px-2.5 py-1.5 text-subtle",
        className
      )}
    />
  );
}

function WireCardBody({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      {...props}
      data-slot="wire-card-body"
      className={cn("px-3 py-2 font-mono text-eyebrow", className)}
    />
  );
}

function WireCardFoot({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      {...props}
      data-slot="wire-card-foot"
      className={cn(
        "flex items-center gap-1.5 border-t border-line bg-canvas px-2.5 py-1.5",
        className
      )}
    />
  );
}

export { WireCard, WireCardHead, WireCardBody, WireCardFoot };
