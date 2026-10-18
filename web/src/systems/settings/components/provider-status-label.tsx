import { cn, Pill } from "@compozy/ui";

import type { ProviderStateView } from "../lib/provider-state";

/**
 * Provider readiness as a plain status readout: dot + muted label, never a
 * plate. Ready stays quiet (neutral dot) so only providers that need attention
 * carry signal color — on the dot, not the text.
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
    <Pill
      className={cn("min-w-0 truncate", className)}
      data-state={dataState}
      data-testid={testId}
      form="plain"
      tone={ready ? "neutral" : tone}
    >
      <Pill.Dot />
      {label}
    </Pill>
  );
}
