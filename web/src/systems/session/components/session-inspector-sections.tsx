import { Coins } from "lucide-react";

import { Collapsible, CollapsibleContent, Metric, MetricGrid } from "@compozy/ui";
import { describeCost } from "@/lib/cost-provenance";

import { hasReportableUsage } from "./session-inspector.logic";
import {
  SessionInspectorDisclosureHead,
  SessionInspectorEmpty,
  SessionInspectorSection,
  SessionInspectorSectionHead,
} from "./session-inspector-section";
import type { InspectorUsage } from "./session-inspector-types";

function formatNumber(value?: number): string {
  if (typeof value !== "number" || !Number.isFinite(value)) return "—";
  return value.toLocaleString();
}

/** Tiles are cards (`card` + the ring in `shadow-card`), one step above the recessed rail. */
const TILE =
  "gap-1 bg-card px-3 py-2.5 shadow-card [&_[data-slot=metric-subtext]]:text-eyebrow [&_[data-slot=metric-subtext]]:whitespace-normal [&_[data-slot=metric-subtext]]:text-subtle";

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
    <SessionInspectorSection data-testid="session-inspector-usage">
      <SessionInspectorSectionHead
        meta={
          turnCount > 0 ? (
            <span data-testid="session-inspector-usage-turns">
              {`across ${turnCount.toLocaleString()} turn${turnCount === 1 ? "" : "s"}`}
            </span>
          ) : undefined
        }
      >
        Tokens & cost
      </SessionInspectorSectionHead>
      {hasUsage ? (
        <>
          <MetricGrid
            columns={2}
            className="grid-cols-2 gap-2"
            data-testid="session-inspector-usage-grid"
          >
            <Metric
              size="compact"
              className={TILE}
              data-testid="session-inspector-usage-cost"
              label="Total cost"
              subtext={cost.note ?? undefined}
              value={
                // The provenance word is a sentence, never dressed as an amount.
                cost.isAmount ? (
                  cost.value
                ) : (
                  <span className="text-small-body font-medium text-fg">{cost.value}</span>
                )
              }
            />
            <Metric
              size="compact"
              className={TILE}
              data-testid="session-inspector-usage-total-tokens"
              label="Total tokens"
              value={formatNumber(usage?.totalTokens)}
            />
          </MetricGrid>
          {/* The in/out/cache split is for the curious: one fold, closed by default. */}
          <Collapsible>
            <SessionInspectorDisclosureHead data-testid="session-inspector-usage-breakdown-toggle">
              Token breakdown
            </SessionInspectorDisclosureHead>
            <CollapsibleContent>
              <MetricGrid
                columns={2}
                className="grid-cols-2 gap-2 pt-2"
                data-testid="session-inspector-usage-breakdown"
              >
                <Metric
                  size="compact"
                  className={TILE}
                  data-testid="session-inspector-usage-tokens-in"
                  label="Tokens in"
                  value={formatNumber(usage?.tokensIn)}
                />
                <Metric
                  size="compact"
                  className={TILE}
                  data-testid="session-inspector-usage-tokens-out"
                  label="Tokens out"
                  value={formatNumber(usage?.tokensOut)}
                />
                {usage?.cacheReadTokens != null ? (
                  <Metric
                    size="compact"
                    className={TILE}
                    label="Cache read"
                    value={formatNumber(usage.cacheReadTokens)}
                  />
                ) : null}
                {usage?.cacheWriteTokens != null ? (
                  <Metric
                    size="compact"
                    className={TILE}
                    label="Cache write"
                    value={formatNumber(usage.cacheWriteTokens)}
                  />
                ) : null}
              </MetricGrid>
            </CollapsibleContent>
          </Collapsible>
        </>
      ) : (
        <SessionInspectorEmpty
          data-testid="session-inspector-usage-empty"
          description="Token counts and cost land here once the agent reports its first turn."
          icon={Coins}
          title="No usage yet"
        />
      )}
    </SessionInspectorSection>
  );
}
