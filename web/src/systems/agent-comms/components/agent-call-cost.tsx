/**
 * What a call cost.
 *
 * Delegation cost is not a field on the call record, and it should not be: a
 * call's work happens in its child session, and wake-consuming deliveries are
 * accounted on the same owner-keyed substrate as every other activation
 * (ADR-004). So this reads the child session's usage — one set of books, no
 * parallel accounting.
 *
 * Every string here comes out of `describeCost()` verbatim: the `≈` prefix when
 * a number is estimated, "Included" under a subscription, "Unavailable" when the
 * provider sent nothing, and `—` when there is no cost status at all. Absent
 * provider data is never rendered as `0` — a fabricated zero is the one failure
 * mode that makes a cost display worse than no cost display.
 */
import { Metric } from "@compozy/ui";

import { describeCost, type CostInput } from "@/lib/cost-provenance";

export interface AgentCallCostProps {
  /** Usage for the call's child session, or an empty object when unknown. */
  usage: CostInput;
  label?: string;
  "data-testid"?: string;
}

export function AgentCallCost({
  usage,
  label = "This call",
  "data-testid": testId,
}: AgentCallCostProps) {
  const cost = describeCost(usage);
  return (
    <Metric
      data-testid={testId}
      data-cost-status={cost.status ?? "none"}
      label={label}
      labelCase="eyebrow"
      size="compact"
      value={cost.value}
      subtext={cost.note ?? undefined}
    />
  );
}
