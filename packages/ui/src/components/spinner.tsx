import type * as React from "react";
import { Loader2Icon } from "lucide-react";

import { cn } from "../lib/utils";

// The standalone default is 16 px, set as `w-4 h-4` rather than `size-4` on
// purpose: a host's `[&_svg:not([class*='size-'])]` glyph ladder (Button, menu
// items) still reaches the spinner and sizes it like any other icon. An
// explicit `size-*` from the caller wins everywhere.
function Spinner({ className, ...props }: React.ComponentProps<"svg">) {
  return (
    <Loader2Icon
      role="status"
      aria-label="Loading"
      className={cn("h-4 w-4 animate-spin", className)}
      {...props}
    />
  );
}

export { Spinner };
