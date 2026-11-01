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
        <span className="text-meta text-muted-foreground">
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
        <span className="ml-auto text-meta text-muted-foreground tabular-nums">
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
      <dt className="shrink-0 text-meta text-muted-foreground">{label}</dt>
      <dd className="min-w-0 truncate text-right text-meta text-fg tabular-nums" title={value}>
        {value}
      </dd>
    </div>
  );
}

function SourceFacts({ target, now }: { target: ProviderUsageTarget; now: number }) {
  const facts: Array<{ label: string; value: string }> = [];
  if (target.account?.label) facts.push({ label: "Account", value: target.account.label });
  if (target.account?.plan) facts.push({ label: "Plan", value: formatPlan(target.account.plan) });
  if (target.source) facts.push({ label: "Source", value: target.source.label });
  const measured = showsUsageData(target) ? formatAgo(target.measured_at, now) : null;
  if (measured) facts.push({ label: "Measured", value: measured });
  if (facts.length === 0) return null;
  return (
    <dl
      aria-label={`${target.provider_name} source`}
      className="flex flex-col gap-1 border-t border-line-soft pt-2"
    >
      {facts.map(fact => (
        <FactRow key={fact.label} label={fact.label} value={fact.value} />
      ))}
    </dl>
  );
}

function MoneyFacts({ target, now }: { target: ProviderUsageTarget; now: number }) {
  if (target.spend_caps.length === 0 && target.balances.length === 0) return null;
  return (
    <dl aria-label={`${target.provider_name} spend`} className="flex flex-col gap-1">
      {target.spend_caps.map(cap => (
        <FactRow key={`cap-${cap.id}`} label={cap.label} value={spendCapText(cap, now)} />
      ))}
      {target.balances.map(balance => (
        <FactRow key={`balance-${balance.id}`} label={balance.label} value={balanceText(balance)} />
      ))}
    </dl>
  );
}

/**
 * Every window, then money in its own units (balances and spend caps, never
 * summed), then who and where the numbers come from and when they were
 * measured, and the provider's error message when the row failed.
 */
export function ProviderUsageDetail({ target, now, className }: ProviderUsageDetailProps) {
  const hasData = showsUsageData(target);
  const stale = target.status === "stale";
  const errorMessage = target.status === "error" || stale ? target.error?.message.trim() : "";

  return (
    <div data-slot="provider-usage-detail" className={cn("flex min-w-0 flex-col gap-3", className)}>
      {hasData && target.windows.length > 0 ? (
        <ul aria-label={`${target.provider_name} limits`} className="flex flex-col gap-2.5">
          {target.windows.map(window => (
            <UsageWindowLine key={window.id} window={window} now={now} stale={stale} />
          ))}
        </ul>
      ) : null}
      {hasData ? <MoneyFacts target={target} now={now} /> : null}
      {errorMessage ? (
        <p data-slot="provider-usage-error" className="text-meta text-pretty text-muted-foreground">
          {errorMessage}
        </p>
      ) : null}
      <SourceFacts target={target} now={now} />
    </div>
  );
}
