import { cn, Pill } from "@compozy/ui";

import type { ProviderStateView } from "../lib/provider-state";

const LABEL_TONE_CLASS: Partial<Record<ProviderStateView["tone"], string>> = {
  warning: "text-warning",
  danger: "text-danger",
};

/**
 * Provider readiness as dot + label. Ready stays quiet (neutral dot, muted
 * text) so only providers that need attention carry signal color.
 */
export function ProviderStatusLabel({
  label,
  tone,
  ready,
  className,
  "data-testid": testId,
  "data-state": dataState,
}: {
  label: string;
  tone: ProviderStateView["tone"];
  ready: boolean;
  className?: string;
  "data-testid"?: string;
  "data-state"?: string;
}) {
  return (
    <span
      className={cn(
        "inline-flex min-w-0 items-center gap-1.5 truncate text-form-label font-medium",
        ready ? "text-muted" : (LABEL_TONE_CLASS[tone] ?? "text-muted"),
        className
      )}
      data-state={dataState}
      data-testid={testId}
    >
      <Pill.Dot tone={ready ? "neutral" : tone} />
      {label}
    </span>
  );
}
