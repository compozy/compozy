import type { ComponentProps } from "react";

import { cn } from "@compozy/ui";

/** Quick-pick chip: quiet at rest, the pressed plate when chosen (pill-group grammar). */
export function ChoiceChip({
  pressed,
  className,
  type = "button",
  ...props
}: ComponentProps<"button"> & { pressed?: boolean }) {
  return (
    <button
      aria-pressed={pressed}
      className={cn(
        "inline-flex items-center gap-1.5 rounded-pill border border-transparent px-2.5 py-1 text-form-label font-medium transition-colors outline-none focus-visible:shadow-focus-ring",
        pressed ? "bg-surface-2 text-fg" : "text-muted hover:bg-surface-2 hover:text-fg",
        className
      )}
      type={type}
      {...props}
    />
  );
}
