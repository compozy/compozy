import { CircleArrowUp } from "lucide-react";

import { Icon } from "@compozy/ui";

import { cn } from "@/lib/utils";

export interface MenubarUpdateIndicatorProps {
  /** Daemon truth: at least one track offers an update and nothing is running. */
  available: boolean;
  /** Navigates to Settings → General, where the offer's detail lives. */
  onActivate: () => void;
}

/**
 * The menubar's update indicator (ADR-006 S2).
 *
 * It exists only while an update is genuinely on offer — and is removed from the
 * DOM, not hidden with CSS, the rest of the time, which is almost always. It
 * carries no count and opens no menu: the bar's job is to say something is
 * waiting, and which track, which versions, and what to do about it belong to
 * the Updates section. Progress, staging, and failure never reach here.
 */
export function MenubarUpdateIndicator({ available, onActivate }: MenubarUpdateIndicatorProps) {
  if (!available) return null;

  return (
    <button
      aria-label="Update available"
      className={cn(
        "grid size-7 place-items-center rounded-md text-info",
        "transition-colors duration-base hover:bg-btn-default-fill",
        "focus-visible:shadow-focus-ring focus-visible:outline-none"
      )}
      data-slot="os-menubar-update"
      data-testid="os-menubar-update"
      onClick={onActivate}
      title="Update available"
      type="button"
    >
      <Icon as={CircleArrowUp} size="lg" />
    </button>
  );
}
