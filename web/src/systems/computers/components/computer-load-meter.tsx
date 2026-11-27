import {
  cn,
  Progress,
  ProgressLabel,
  ProgressValue,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@compozy/ui";

import type { ComputerLoadFixture } from "../mocks/remote-computers-fixtures";

const GB = 1024 * 1024 * 1024;

function formatGb(bytes: number): string {
  const gb = bytes / GB;
  return gb >= 100 ? `${Math.round(gb)}` : gb.toFixed(1);
}

export interface ComputerLoadMeterProps {
  load: ComputerLoadFixture | null;
  /** Optional computer name for accessible summary. */
  computerName?: string;
  className?: string;
}

/**
 * Host capacity indicator (`research-product` §3.2): compact 36px horizontal
 * fill bar + tabular `<N>% load` text so it reads unmistakably as machine load
 * (never Wi-Fi signal), expanding to a CPU + memory tooltip on hover/focus.
 * Unsampled or stale telemetry reads "Unsampled", never "0%".
 */
export function ComputerLoadMeter({ load, computerName, className }: ComputerLoadMeterProps) {
  if (!load || load.stale) {
    return (
      <span
        className={cn(
          "inline-flex items-center gap-1.5 text-eyebrow text-faint tabular-nums",
          className
        )}
        data-slot="computer-load-meter"
        data-state="unsampled"
      >
        <span
          aria-hidden="true"
          className="inline-block h-1 w-9 shrink-0 rounded-full bg-surface-2"
        />
        <span>Unsampled</span>
      </span>
    );
  }

  const cpuPct = Math.max(0, Math.min(100, Math.round(load.cpuUtil * 100)));
  const memUsedBytes = Math.max(0, load.memTotalBytes - load.memAvailableBytes);
  const memUsedRatio = load.memTotalBytes > 0 ? memUsedBytes / load.memTotalBytes : 0;
  const memPct = Math.max(0, Math.min(100, Math.round(memUsedRatio * 100)));
  const peakPct = Math.max(cpuPct, memPct);
  const subject = computerName ? `${computerName} load` : "Computer load";

  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <button
            aria-label={`${subject}: ${peakPct}% load (CPU ${cpuPct}%, memory ${memPct}%)`}
            className={cn(
              "inline-flex cursor-default items-center gap-1.5 rounded-xs text-eyebrow text-muted-foreground tabular-nums transition-colors duration-base hover:text-fg focus-visible:shadow-focus-ring focus-visible:outline-none",
              className
            )}
            data-slot="computer-load-meter"
            data-state="sampled"
            type="button"
          />
        }
      >
        <span
          aria-hidden="true"
          className="relative inline-flex h-1 w-9 shrink-0 overflow-hidden rounded-full bg-surface-2"
        >
          <span
            className="h-full rounded-full bg-fg/75 transition-[width] duration-slow ease-out"
            style={{ width: `${Math.max(6, peakPct)}%` }}
          />
        </span>
        <span>{peakPct}% load</span>
      </TooltipTrigger>
      <TooltipContent
        align="center"
        className="w-60 flex-col items-stretch gap-2.5 p-3 font-normal"
        side="top"
      >
        <Progress tone="brand" value={cpuPct}>
          <ProgressLabel className="text-eyebrow">CPU</ProgressLabel>
          <ProgressValue className="text-eyebrow">
            {() => `${cpuPct}% · ${load.cpuCount} cores`}
          </ProgressValue>
        </Progress>
        <Progress tone="brand" value={memPct}>
          <ProgressLabel className="text-eyebrow">Memory</ProgressLabel>
          <ProgressValue className="text-eyebrow">
            {() => `${formatGb(load.memAvailableBytes)} / ${formatGb(load.memTotalBytes)} GB free`}
          </ProgressValue>
        </Progress>
        <div className="flex items-center justify-between border-t border-line-soft pt-1.5 text-badge text-faint tabular-nums">
          <span>Host capacity</span>
          <span>Sampled {load.sampledAgoLabel}</span>
        </div>
      </TooltipContent>
    </Tooltip>
  );
}
