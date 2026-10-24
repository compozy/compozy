import { Gauge } from "lucide-react";

import {
  cn,
  Icon,
  Progress,
  Skeleton,
  StatusDot,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@compozy/ui";

import { useUsageClock } from "../hooks/use-usage-clock";
import { clampPercent } from "../lib/format";
import { toneForSeverity } from "../lib/severity";
import { formatTrayLabel } from "../lib/view-model";
import type { ProviderUsageTraySignal } from "../types";

export interface ProviderUsageTrayMeterProps extends Omit<
  React.ComponentProps<"button">,
  "children"
> {
  /** The daemon's tightest known window; `null` shows the glyph without a meter. */
  tray: ProviderUsageTraySignal | null;
  /** When the tray window was measured, for the stale tooltip ("updated 25 min ago"). */
  measuredAt?: string | null;
  /**
   * The first usage read has not answered. The control holds its slot with the meter's shape,
   * unmeasured, and stays transparent for a beat (as a skeleton does), so an answer that comes
   * quickly never flashes it and the controls beside it do not move when it lands.
   */
  loading?: boolean;
}

/**
 * The topbar's Provider usage control: a gauge glyph over a compact bar filled
 * to the tightest window. The host owns the footprint: the menubar passes its
 * tray icon-button classes through `className`, so the control matches All
 * desktops and the bell.
 * Its accessible name and tooltip carry the window, percent, and reset; the
 * bar tone is never the only signal. Stale data dims the bar and adds a ring
 * marker. Hand it to a popover trigger's `render`; props and `ref` reach the
 * button.
 */
export function ProviderUsageTrayMeter({
  tray,
  measuredAt = null,
  loading = false,
  className,
  ref,
  ...props
}: ProviderUsageTrayMeterProps) {
  const now = useUsageClock();
  const label = formatTrayLabel(tray, measuredAt, now);
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <button
            ref={ref}
            type="button"
            aria-label={label}
            aria-haspopup="dialog"
            data-slot="provider-usage-tray-meter"
            data-severity={tray?.severity}
            data-stale={tray?.stale || undefined}
            data-loading={loading || undefined}
            className={cn(className, loading && "animate-skeleton-reveal")}
            {...props}
          >
            {tray || loading ? (
              <span aria-hidden="true" className="flex flex-col items-center gap-0.75">
                <Icon as={Gauge} className="size-3.5" />
                {tray ? (
                  <Progress
                    value={clampPercent(tray.used_percent)}
                    tone={toneForSeverity(tray.severity)}
                    className={cn(
                      "w-4 [&_[data-slot=progress-track]]:h-0.75",
                      tray.stale && "opacity-50"
                    )}
                  />
                ) : (
                  <Skeleton className="h-0.75 w-4 rounded-pill" />
                )}
              </span>
            ) : (
              <Icon as={Gauge} aria-hidden="true" className="size-4" />
            )}
            {tray?.stale ? (
              <StatusDot
                tone="faint"
                variant="ring"
                size="sm"
                className="absolute top-1.5 right-1.5"
              />
            ) : null}
          </button>
        }
      />
      <TooltipContent side="bottom">{label}</TooltipContent>
    </Tooltip>
  );
}
