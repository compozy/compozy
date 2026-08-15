import type { ComponentProps } from "react";

import { cn } from "@compozy/ui";

type TriggerValueBadgeProps = Omit<ComponentProps<"span">, "children" | "value"> & {
  fill?: boolean;
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
export function TriggerValueBadge({
  fill = false,
  value,
  className,
  ...props
}: TriggerValueBadgeProps) {
  return (
    <span
      className={cn(
        "rounded-xs bg-badge-fill px-1.5 py-px font-mono text-badge text-subtle",
        fill && "block w-full truncate",
        className
      )}
      {...props}
    >
      {value}
    </span>
  );
}
