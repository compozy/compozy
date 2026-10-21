import { Gauge } from "lucide-react";

import {
  cn,
  Icon,
  Progress,
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
}

/** The tray icon-button footprint (34 px pill, muted glyph) the topbar uses. */
const TRAY_BUTTON = [
  "relative grid size-8.5 place-items-center rounded-pill text-muted",
  "transition-colors duration-base hover:bg-rail-hover hover:text-fg",
  "data-popup-open:bg-rail-selected data-popup-open:text-fg",
  "focus-visible:shadow-focus-ring focus-visible:outline-none",
].join(" ");

/**
 * The topbar's Provider usage control: a gauge glyph over a compact bar filled
 * to the tightest window, inside the same footprint as the other tray buttons.
 * Its accessible name and tooltip carry the window, percent, and reset; the
 * bar tone is never the only signal. Stale data dims the bar and adds a ring
 * marker. Hand it to a popover trigger's `render`; props and `ref` reach the
 * button.
 */
export function ProviderUsageTrayMeter({
  tray,
  measuredAt = null,
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
            className={cn(TRAY_BUTTON, className)}
            {...props}
          >
            {tray ? (
              <span aria-hidden="true" className="flex flex-col items-center gap-0.75">
                <Icon as={Gauge} className="size-3.5" />
                <Progress
                  value={clampPercent(tray.used_percent)}
                  tone={toneForSeverity(tray.severity)}
                  className={cn(
                    "w-4 [&_[data-slot=progress-track]]:h-0.75",
                    tray.stale && "opacity-50"
                  )}
                />
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
