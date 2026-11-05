import { cn, Pill } from "@compozy/ui";

import type { ProviderStateView } from "../lib/provider-state";

/**
 * Provider readiness as a plain status readout: dot + muted label, never a
 * plate. Signal color lives on the dot only, so Ready (success) stays easy to
 * scan next to providers that need attention without louder text.
 */
export function ProviderStatusLabel({
  label,
  tone,
  className,
  "data-testid": testId,
  "data-state": dataState,
}: {
  label: string;
  tone: ProviderStateView["tone"];
  className?: string;
  "data-testid"?: string;
  "data-state"?: string;
}) {
  return (
    <Pill
      className={cn("min-w-0 truncate", className)}
      data-state={dataState}
      data-testid={testId}
      form="plain"
      tone={tone}
    >
      <Pill.Dot />
      {label}
    </Pill>
  );
}
