import { Pill } from "@compozy/ui";

import type { GatewaySignalTone } from "../lib/gateway-copy";

export interface GatewayStatusChipProps {
  label: string;
  tone: GatewaySignalTone;
  /**
   * Screen-reader text carrying the same distinction the colour makes, so the
   * state never depends on hue alone.
   */
  detail?: string;
  "data-testid"?: string;
}

/**
 * Status chip for any gateway state that comes from the daemon. Tint tokens
 * only — a reachability state is information, not a banner.
 */
export function GatewayStatusChip({
  label,
  tone,
  detail,
  "data-testid": testId,
}: GatewayStatusChipProps) {
  return (
    <Pill data-testid={testId} size="sm" tone={tone}>
      {label}
      {detail ? <span className="sr-only"> — {detail}</span> : null}
    </Pill>
  );
}
