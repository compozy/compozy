import type { ComponentProps } from "react";
import { CornerDownRight } from "lucide-react";

import { cn } from "@compozy/ui";

export interface ProfileDestinationChipProps extends Omit<ComponentProps<"span">, "children"> {
  /** The profile this creation lands in. Fixed text — never a control (ADR-005). */
  profile: string;
}

/**
 * States where a new item will be filed while the aggregate is on.
 *
 * It is a label, not a picker: turning every shared creation surface into a
 * scope-aware control was rejected outright, and the owner-naming toast after
 * the commit is what surfaces a misfile. It sits in the default read of the
 * surface — a tooltip would be too easy to miss to be a guardrail at all.
 */
export function ProfileDestinationChip({
  profile,
  className,
  ...props
}: ProfileDestinationChipProps) {
  return (
    <span
      data-slot="profile-destination-chip"
      data-testid="profile-destination-chip"
      className={cn(
        "inline-flex h-5 shrink-0 items-center gap-1.5 rounded-xs bg-badge-fill px-2",
        "text-eyebrow font-medium tracking-eyebrow text-muted",
        className
      )}
      {...props}
    >
      <CornerDownRight aria-hidden="true" className="size-2.5" />
      <b className="font-medium text-fg">{profile}</b>
    </span>
  );
}
