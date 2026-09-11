import { Checkbox as CheckboxPrimitive } from "@base-ui/react/checkbox";
import { CheckIcon, MinusIcon } from "lucide-react";

import { cn } from "../lib/utils";

/** Render checked and mixed states through Base UI while preserving native checkbox props. */
function Checkbox({ className, indeterminate, ...props }: CheckboxPrimitive.Root.Props) {
  return (
    <CheckboxPrimitive.Root
      data-slot="checkbox"
      indeterminate={indeterminate}
      className={cn(
        "peer relative flex size-4 shrink-0 items-center justify-center rounded-xs border border-line bg-elevated transition-colors outline-none after:absolute after:-inset-x-3 after:-inset-y-2 group-has-disabled/field:opacity-50 focus-visible:outline-none focus-visible:shadow-focus-ring disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50 aria-invalid:border-danger data-checked:border-accent data-checked:bg-accent data-checked:text-accent-ink",
        "data-indeterminate:border-accent data-indeterminate:bg-accent data-indeterminate:text-accent-ink",
        className
      )}
      {...props}
    >
      <CheckboxPrimitive.Indicator
        data-slot="checkbox-indicator"
        className="grid place-content-center text-current transition-none [&>svg]:size-3.5"
      >
        {indeterminate ? <MinusIcon /> : <CheckIcon />}
      </CheckboxPrimitive.Indicator>
    </CheckboxPrimitive.Root>
  );
}

export { Checkbox };
