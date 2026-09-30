import type * as React from "react";

import { cn } from "../lib/utils";

function Kbd({ className, ...props }: React.ComponentProps<"kbd">) {
  return (
    <kbd
      data-slot="kbd"
      className={cn(
        "pointer-events-none inline-flex h-5 w-fit min-w-5 items-center justify-center gap-1 rounded-xs bg-surface-2 px-1.5 font-keys text-kbd tracking-kbd text-fg-2 inset-ring inset-ring-line select-none [&_svg:not([class*='size-'])]:size-3",
        // Inside an inverted tooltip or a button the cap drops its plate and
        // reads as a quiet trailing hint in the host's own ink.
        "in-data-[slot=tooltip-content]:min-w-0 in-data-[slot=tooltip-content]:bg-transparent in-data-[slot=tooltip-content]:px-0 in-data-[slot=tooltip-content]:text-current in-data-[slot=tooltip-content]:opacity-60 in-data-[slot=tooltip-content]:inset-ring-0",
        "in-data-[slot=button]:min-w-0 in-data-[slot=button]:bg-transparent in-data-[slot=button]:px-0 in-data-[slot=button]:text-current in-data-[slot=button]:opacity-80 in-data-[slot=button]:inset-ring-0",
        className
      )}
      {...props}
    />
  );
}

function KbdGroup({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <kbd
      data-slot="kbd-group"
      className={cn(
        "inline-flex items-center gap-1 in-data-[slot=button]:gap-px in-data-[slot=tooltip-content]:gap-px",
        className
      )}
      {...props}
    />
  );
}

export { Kbd, KbdGroup };
