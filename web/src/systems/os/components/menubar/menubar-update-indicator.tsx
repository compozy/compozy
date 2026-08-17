import { Download } from "lucide-react";

import { Button, Icon } from "@compozy/ui";

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
    <Button
      aria-label="Update available"
      className="size-7"
      data-slot="os-menubar-update"
      data-testid="os-menubar-update"
      onClick={onActivate}
      size="icon"
      title="Update available"
      type="button"
      variant="default"
    >
      <Icon as={Download} size="lg" />
    </Button>
  );
}
