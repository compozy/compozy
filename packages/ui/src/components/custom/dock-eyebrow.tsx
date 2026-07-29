"use client";

import * as React from "react";

import { cn } from "../../lib/utils";
import { Eyebrow } from "./eyebrow";

/** Uppercase kicker naming the decision kind ("Permission", "Question"). */
function DockEyebrow({ children, className, ...props }: React.ComponentProps<"span">) {
  return (
    <Eyebrow data-slot="dock-eyebrow" className={cn("text-subtle", className)} {...props}>
      {children}
    </Eyebrow>
  );
}

export { DockEyebrow };
