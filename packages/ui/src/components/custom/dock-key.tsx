"use client";

import * as React from "react";

import { cn } from "../../lib/utils";

/** Keyboard shortcut chip on a dock action button ("1"–"4"). */
function DockKey({ children, className, ...props }: React.ComponentProps<"kbd">) {
  return (
    <kbd
      data-slot="dock-key"
      className={cn("ml-[3px] font-mono text-[9px] text-current opacity-60", className)}
      {...props}
    >
      {children}
    </kbd>
  );
}

export { DockKey };
