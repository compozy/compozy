import type { ReactNode } from "react";

import { cn, Pill, Progress, ProgressLabel, ProgressValue, StackedProgress } from "@compozy/ui";

const MAX_DISCRETE_SEGMENTS = 10;

export interface PlanUsageMeterProps {
  label: ReactNode;
  /** Number of slots currently in use (or active computers). */
  used: number;
  /**
   * Plan cap for this resource:
   * - `null` = unlimited (renders count + muted Unlimited label, never a fake 100% bar)
   * - `0` = locked / not included on plan (renders "Part of Pro" instead of "0 of 0")
   * - `1..10` = discrete pill segments (one segment per slot)
   * - `>10` = continuous Progress bar
   */
  limit: number | null;
  /** Plural noun for contextual readout, e.g. "remote computers" or "hosted routes". */
  unitLabel?: string;
  /** Copy when `limit === 0`, default "Part of Pro". */
  lockedLabel?: string;
  /** Copy when `limit === null`, default "Unlimited on Pro". */
  unlimitedLabel?: string;
  /**
   * When downgraded, renders a stacked breakdown showing `used` active +
   * `needsProCount` limited computers instead of a single fill bar.
   */
  needsProCount?: number;
  /** Optional inline CTA (e.g. Upgrade button) aligned to the right. */
  action?: ReactNode;
  /** Compact inline layout for list header lines (thin bar + readout + action on one line). */
  compact?: boolean;
  className?: string;
}

/**
 * Contextual plan-quota meter (`research-product` §3.2, `research-references` §4.1):
 * - Neutral foreground / brand fill for normal usage (amber is reserved for attention)
 * - Discrete pill segments for caps `<= 10`, smooth `Progress` bar above 10
 * - `StackedProgress` (`brand` active + muted `neutral` needs Pro) when downgraded
 * - Honest `Part of Pro` when `limit === 0`, and `Unlimited` when `limit === null`
 */
export function PlanUsageMeter({
  label,
  used,
  limit,
  unitLabel,
  lockedLabel = "Part of Pro",
  unlimitedLabel = "Unlimited on Pro",
  needsProCount = 0,
  action,
  compact = false,
  className,
}: PlanUsageMeterProps) {
  const unitSuffix = unitLabel ? ` ${unitLabel}` : "";
  const ariaPrefix = typeof label === "string" ? label : "Plan usage";

  if (limit === 0) {
    return (
      <div
        className={cn(
          "flex min-w-0 flex-wrap items-center justify-between gap-2",
          compact ? "text-meta" : "text-small-body",
          className
        )}
        data-slot="plan-usage-meter"
        data-state="locked"
      >
        {label ? <span className="font-medium text-fg">{label}</span> : null}
        <div className="flex items-center gap-2">
          <Pill size="xs" tone="brand">
            {lockedLabel}
          </Pill>
          {action}
        </div>
      </div>
    );
  }

  if (needsProCount > 0) {
    const total = used + needsProCount;
    const summary = `${used} active · ${needsProCount} need Pro`;

    if (compact) {
      return (
        <div
          className={cn("flex min-w-0 flex-wrap items-center gap-2.5", className)}
          data-slot="plan-usage-meter"
          data-state="downgraded"
        >
          <StackedProgress
            ariaLabel={`${ariaPrefix}: ${summary}`}
            className="h-1 w-20 shrink-0"
            segments={[
              { value: used, tone: "brand", label: "Active" },
              { value: needsProCount, tone: "neutral", label: "Needs Pro" },
            ]}
            total={total}
          />
          <span className="text-meta text-muted-foreground tabular-nums">{summary}</span>
          {action}
        </div>
      );
    }

    return (
      <div
        className={cn("flex min-w-0 flex-col gap-2", className)}
        data-slot="plan-usage-meter"
        data-state="downgraded"
      >
        <div className="flex flex-wrap items-center justify-between gap-2">
          <span className="text-small-body font-medium text-fg">{label}</span>
          <div className="flex items-center gap-2.5">
            <span className="text-meta text-muted-foreground tabular-nums">{summary}</span>
            {action}
          </div>
        </div>
        <StackedProgress
          ariaLabel={`${ariaPrefix}: ${summary}`}
          className="h-1"
          segments={[
            { value: used, tone: "brand", label: "Active" },
            { value: needsProCount, tone: "neutral", label: "Needs Pro" },
          ]}
          total={total}
        />
      </div>
    );
  }

  if (limit === null) {
    return (
      <div
        className={cn(
          "flex min-w-0 flex-wrap items-center justify-between gap-2",
          compact ? "text-meta" : "text-small-body",
          className
        )}
        data-slot="plan-usage-meter"
        data-state="unlimited"
      >
        {label ? <span className="font-medium text-fg">{label}</span> : null}
        <div className="flex items-center gap-2">
          <span className="text-meta text-muted-foreground tabular-nums">
            {used}
            {unitSuffix}
          </span>
          <span aria-hidden="true" className="size-0.5 rounded-full bg-faint" />
          <span className="text-meta text-subtle">{unlimitedLabel}</span>
          {action}
        </div>
      </div>
    );
  }

  const readout = `${used} of ${limit}${unitSuffix}`;

  if (limit <= MAX_DISCRETE_SEGMENTS) {
    const fillClass = used >= limit ? "bg-brand" : "bg-fg";

    if (compact) {
      return (
        <div
          className={cn("flex min-w-0 flex-wrap items-center gap-2.5", className)}
          data-slot="plan-usage-meter"
          data-state={used >= limit ? "at-limit" : "within-limit"}
        >
          <div
            aria-label={`${ariaPrefix}: ${readout}`}
            aria-valuemax={limit}
            aria-valuemin={0}
            aria-valuenow={used}
            className="flex w-16 shrink-0 items-center gap-0.5"
            role="meter"
          >
            {Array.from({ length: limit }, (_, index) => {
              const filled = index < used;
              return (
                <span
                  aria-hidden="true"
                  className={cn(
                    "h-1 flex-1 rounded-full transition-colors duration-base",
                    filled ? fillClass : "bg-surface-2"
                  )}
                  data-filled={filled ? "true" : "false"}
                  key={index}
                />
              );
            })}
          </div>
          <span className="text-meta text-muted-foreground tabular-nums">{readout}</span>
          {action}
        </div>
      );
    }

    return (
      <div
        className={cn("flex min-w-0 flex-col gap-2", className)}
        data-slot="plan-usage-meter"
        data-state={used >= limit ? "at-limit" : "within-limit"}
      >
        <div className="flex flex-wrap items-center justify-between gap-2">
          <span className="text-small-body font-medium text-fg">{label}</span>
          <div className="flex items-center gap-2.5">
            <span className="text-meta text-muted-foreground tabular-nums">{readout}</span>
            {action}
          </div>
        </div>
        <div
          aria-label={`${ariaPrefix}: ${readout}`}
          aria-valuemax={limit}
          aria-valuemin={0}
          aria-valuenow={used}
          className="flex w-full items-center gap-1"
          role="meter"
        >
          {Array.from({ length: limit }, (_, index) => {
            const filled = index < used;
            return (
              <span
                aria-hidden="true"
                className={cn(
                  "h-1 flex-1 rounded-full transition-colors duration-base",
                  filled ? fillClass : "bg-surface-2"
                )}
                data-filled={filled ? "true" : "false"}
                key={index}
              />
            );
          })}
        </div>
      </div>
    );
  }

  const pct = Math.max(0, Math.min(100, Math.round((used / limit) * 100)));

  if (compact) {
    return (
      <div
        className={cn("flex min-w-0 flex-wrap items-center gap-2.5", className)}
        data-slot="plan-usage-meter"
        data-state={used >= limit ? "at-limit" : "within-limit"}
      >
        <span
          aria-label={`${ariaPrefix}: ${readout}`}
          aria-valuemax={limit}
          aria-valuemin={0}
          aria-valuenow={used}
          className="relative inline-flex h-1 w-20 shrink-0 overflow-hidden rounded-full bg-surface-2"
          role="meter"
        >
          <span
            aria-hidden="true"
            className="h-full rounded-full bg-brand"
            style={{ width: `${pct}%` }}
          />
        </span>
        <span className="text-meta text-muted-foreground tabular-nums">{readout}</span>
        {action}
      </div>
    );
  }

  return (
    <div
      className={cn("flex min-w-0 flex-col gap-1.5", className)}
      data-slot="plan-usage-meter"
      data-state={used >= limit ? "at-limit" : "within-limit"}
    >
      <div className="flex items-center justify-between gap-2">
        <Progress className="flex-1" tone="brand" value={pct}>
          <ProgressLabel>{label}</ProgressLabel>
          <ProgressValue>{readout}</ProgressValue>
        </Progress>
        {action}
      </div>
    </div>
  );
}
