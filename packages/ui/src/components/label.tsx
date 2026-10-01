import type * as React from "react";

import { cn } from "../lib/utils";

function Label({ className, htmlFor, ...props }: React.ComponentProps<"label">) {
  return (
    <label
      data-slot="label"
      htmlFor={htmlFor}
      className={cn(
        // A label must never outrank the value it names: `text-form-label` and
        // the `Input` value (`--text-form-input`) share the 13px tier; the
        // Tailwind default `text-sm` (14px) inverted that everywhere.
        "flex items-center gap-2 text-form-label leading-none font-medium select-none group-data-[disabled=true]:pointer-events-none group-data-[disabled=true]:opacity-50 peer-disabled:cursor-not-allowed peer-disabled:opacity-50",
        className
      )}
      {...props}
    />
  );
}

export { Label };
