import { Input as InputPrimitive } from "@base-ui/react/input";
import type * as React from "react";

import { cn } from "../lib/utils";

function Input({ className, type, ...props }: React.ComponentProps<"input">) {
  return (
    <InputPrimitive
      type={type}
      data-slot="input"
      className={cn(
        "h-input w-full min-w-0 rounded-md border border-line bg-elevated px-3 py-0 text-small-body text-fg transition-colors outline-none selection:bg-accent-tint-strong selection:text-fg file:inline-flex file:h-6 file:border-0 file:bg-transparent file:text-small-body file:font-medium file:text-fg placeholder:text-subtle focus-visible:outline-none focus-visible:shadow-focus-ring focus-visible:border-line-strong disabled:pointer-events-none disabled:cursor-not-allowed disabled:border-line-soft disabled:bg-canvas disabled:text-disabled disabled:opacity-100 aria-invalid:border-danger aria-invalid:shadow-none",
        className
      )}
      {...props}
    />
  );
}

export { Input };
