import type { ComponentProps } from "react";

import { cn } from "@compozy/ui";

type AutomationValueBadgeProps = Omit<ComponentProps<"span">, "children" | "value"> & {
  fill?: boolean;
  /** `variable` marks a template variable filled in from each event (info tint). */
  tone?: "value" | "variable";
  value: string;
};

/**
 * A literal runtime value inside prose — a matched filter value, a mapped event
 * path, a template variable. Quiet by design: the sentence carries the meaning,
 * the badge only marks where the raw string starts and stops.
 *
 * `fill` stretches it across a value column so a table of mapped inputs reads
 * as a set of fields rather than a scatter of chips.
 */
export function AutomationValueBadge({
  fill = false,
  tone = "value",
  value,
  className,
  ...props
}: AutomationValueBadgeProps) {
  return (
    <span
      className={cn(
        "rounded-xs px-1.5 py-px font-mono text-badge",
        tone === "variable" ? "bg-info-tint text-info" : "bg-surface-2 text-subtle",
        fill && "block w-full truncate",
        className
      )}
      {...props}
    >
      {value}
    </span>
  );
}
