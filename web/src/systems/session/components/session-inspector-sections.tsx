import { Gauge } from "lucide-react";

import { Empty, Eyebrow, Metric, MetricGrid } from "@compozy/ui";
import { describeCost } from "@/lib/cost-provenance";

import { hasReportableUsage } from "./session-inspector.logic";
import type { InspectorUsage } from "./session-inspector-types";

function formatNumber(value?: number): string {
  if (typeof value !== "number" || !Number.isFinite(value)) return "—";
  return value.toLocaleString();
}

export function SessionInspectorUsageSection({
  usage,
}: {
  usage: InspectorUsage | null | undefined;
}) {
  const cost = describeCost({
    status: usage?.costStatus,
    source: usage?.costSource,
    amount: usage?.costUsd,
    currency: usage?.costCurrency,
  });
  const turnCount = usage?.turnCount ?? 0;
  const hasUsage = usage != null && hasReportableUsage(usage, cost);

  return (
    <section className="flex flex-col gap-3" data-testid="session-inspector-usage">
      <div className="flex items-center justify-between gap-2">
        <Eyebrow>Tokens & cost</Eyebrow>
        {turnCount > 0 ? (
          <Eyebrow className="text-subtle self-start" data-testid="session-inspector-usage-turns">
            {`Across ${turnCount.toLocaleString()} turn${turnCount === 1 ? "" : "s"}`}
          </Eyebrow>
        ) : null}
      </div>
      {hasUsage ? (
        <>
          <MetricGrid columns={2} className="gap-2" data-testid="session-inspector-usage-grid">
            <Metric
              size="compact"
              className="p-3"
              data-testid="session-inspector-usage-tokens-in"
              label="Tokens in"
              value={formatNumber(usage?.tokensIn)}
            />
            <Metric
              size="compact"
              className="p-3"
              data-testid="session-inspector-usage-tokens-out"
              label="Tokens out"
              value={formatNumber(usage?.tokensOut)}
            />
            {usage?.cacheReadTokens != null ? (
              <Metric
                size="compact"
                className="p-3"
                label="Cache read"
                value={formatNumber(usage.cacheReadTokens)}
              />
            ) : null}
            {usage?.cacheWriteTokens != null ? (
              <Metric
                size="compact"
                className="p-3"
                label="Cache write"
                value={formatNumber(usage.cacheWriteTokens)}
              />
            ) : null}
            <Metric
              size="compact"
              className="col-span-2 p-3"
              data-testid="session-inspector-usage-total-tokens"
              label="Total tokens"
              value={formatNumber(usage?.totalTokens)}
            />
            <Metric
              size="compact"
              className="col-span-2 p-3"
              data-testid="session-inspector-usage-cost"
              label="Total cost"
              subtext={cost.note ?? undefined}
              value={cost.value}
            />
          </MetricGrid>
        </>
      ) : (
        <Empty
          data-testid="session-inspector-usage-empty"
          description="Token counts and cost land here once the agent reports its first turn."
          icon={Gauge}
          title="No usage yet"
        />
      )}
    </section>
  );
}
