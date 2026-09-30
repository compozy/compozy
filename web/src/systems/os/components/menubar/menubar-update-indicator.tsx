import { Download } from "lucide-react";

import { Button, Icon, StatusDot, Tooltip, TooltipContent, TooltipTrigger } from "@compozy/ui";

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
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            aria-label="Update available"
            className="relative size-8.5 hover:bg-rail-hover"
            data-slot="os-menubar-update"
            data-testid="os-menubar-update"
            onClick={onActivate}
            size="icon"
            type="button"
            variant="quiet"
          >
            <Icon as={Download} size="lg" />
            {/* Quiet offer: a dot, not an accent fill competing with the bell. */}
            <StatusDot
              tone="accent"
              size="sm"
              aria-hidden="true"
              className="absolute top-1 right-1"
            />
          </Button>
        }
      />
      <TooltipContent side="bottom">Update available</TooltipContent>
    </Tooltip>
  );
}
