import { cn, Progress, ProgressLabel } from "@compozy/ui";

import {
  clampPercent,
  formatAgo,
  formatAmount,
  formatPercent,
  formatPlan,
  formatResetsAt,
  formatShortDate,
  formatWindowReset,
} from "../lib/format";
import { hasKnownPercent, severityForPercent, toneForSeverity } from "../lib/severity";
import { showsUsageData } from "../lib/status";
import type {
  ProviderUsageBalance,
  ProviderUsageSpendCap,
  ProviderUsageTarget,
  ProviderUsageWindow,
} from "../types";

export interface ProviderUsageDetailProps {
  target: ProviderUsageTarget;
  now: number;
  className?: string;
}

function joinParts(parts: ReadonlyArray<string | null | undefined>): string {
  return parts.filter((part): part is string => Boolean(part)).join(" · ");
}

function windowAmounts(window: ProviderUsageWindow): string | null {
  if (window.used && window.limit) {
    return `${formatAmount(window.used)} of ${formatAmount(window.limit)}`;
  }
  return window.used ? `${formatAmount(window.used)} used` : null;
}

function UsageWindowLine({
  window,
  now,
  stale,
}: {
  window: ProviderUsageWindow;
  now: number;
  stale: boolean;
}) {
  const reset = formatWindowReset(window, now);
  if (!hasKnownPercent(window)) {
    // Unknown is not zero: no bar, the words say so.
    return (
      <li data-slot="provider-usage-window" data-known="false" className="flex flex-col gap-0.5">
        <span className="text-small-body font-medium text-fg">{window.label}</span>
        <span className="text-meta text-muted">
          {joinParts(["Usage unknown", windowAmounts(window), reset])}
        </span>
      </li>
    );
  }
  const severity = severityForPercent(window.used_percent);
  const usage = `${formatPercent(window.used_percent)} used`;
  return (
    <li data-slot="provider-usage-window" data-known="true" data-severity={severity}>
      <Progress
        value={clampPercent(window.used_percent)}
        tone={toneForSeverity(severity)}
        getAriaValueText={() => usage}
        className={cn("gap-x-2 gap-y-1.5", stale && "[&_[data-slot=progress-track]]:opacity-50")}
      >
        <ProgressLabel>{window.label}</ProgressLabel>
        <span className="ml-auto text-meta text-muted tabular-nums">
          {joinParts([
            severity === "exhausted" ? `${usage} · limit reached` : usage,
            windowAmounts(window),
            reset,
          ])}
        </span>
      </Progress>
    </li>
  );
}

function spendCapText(cap: ProviderUsageSpendCap, now: number): string {
  // A missing limit is "no cap reported", never "unlimited".
  const amount = cap.limit
    ? `${formatAmount(cap.used)} of ${formatAmount(cap.limit)}`
    : `${formatAmount(cap.used)} spent`;
  return joinParts([
    amount,
    cap.enabled === false ? "off" : null,
    formatResetsAt(cap.resets_at, now),
  ]);
}

function balanceText(balance: ProviderUsageBalance): string {
  const expires = formatShortDate(balance.expires_at);
  return joinParts([formatAmount(balance.remaining), expires ? `expires ${expires}` : null]);
}

function FactRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex min-w-0 items-baseline justify-between gap-3">
      <dt className="shrink-0 text-meta text-muted">{label}</dt>
      <dd className="min-w-0 truncate text-right text-meta text-fg tabular-nums" title={value}>
        {value}
      </dd>
    </div>
  );
}

/**
 * Every window, then money in its own units (balances and spend caps, never
 * summed), then who and where the numbers come from and when they were
 * measured, and the provider's error message when the row failed.
 */
export function ProviderUsageDetail({ target, now, className }: ProviderUsageDetailProps) {
  const hasData = showsUsageData(target);
  const windows = hasData ? target.windows : [];
  const balances = hasData ? target.balances : [];
  const spendCaps = hasData ? target.spend_caps : [];
  const plan = target.account?.plan ? formatPlan(target.account.plan) : "";
  const measured = hasData ? formatAgo(target.measured_at, now) : null;
  const errorMessage =
    target.status === "error" || target.status === "stale" ? target.error?.message.trim() : "";
  const stale = target.status === "stale";

  return (
    <div data-slot="provider-usage-detail" className={cn("flex min-w-0 flex-col gap-3", className)}>
      {windows.length > 0 ? (
        <ul aria-label={`${target.provider_name} limits`} className="flex flex-col gap-2.5">
          {windows.map(window => (
            <UsageWindowLine key={window.id} window={window} now={now} stale={stale} />
          ))}
        </ul>
      ) : null}
      {spendCaps.length > 0 || balances.length > 0 ? (
        <dl aria-label={`${target.provider_name} spend`} className="flex flex-col gap-1">
          {spendCaps.map(cap => (
            <FactRow key={`cap-${cap.id}`} label={cap.label} value={spendCapText(cap, now)} />
          ))}
          {balances.map(balance => (
            <FactRow
              key={`balance-${balance.id}`}
              label={balance.label}
              value={balanceText(balance)}
            />
          ))}
        </dl>
      ) : null}
      {errorMessage ? (
        <p data-slot="provider-usage-error" className="text-meta text-pretty text-muted">
          {errorMessage}
        </p>
      ) : null}
      {target.account?.label || plan || target.source || measured ? (
        <dl
          aria-label={`${target.provider_name} source`}
          className="flex flex-col gap-1 border-t border-line-soft pt-2"
        >
          {target.account?.label ? <FactRow label="Account" value={target.account.label} /> : null}
          {plan ? <FactRow label="Plan" value={plan} /> : null}
          {target.source ? <FactRow label="Source" value={target.source.label} /> : null}
          {measured ? <FactRow label="Measured" value={measured} /> : null}
        </dl>
      ) : null}
    </div>
  );
}
