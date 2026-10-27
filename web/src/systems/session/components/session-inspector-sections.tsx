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
import { SessionInspectorUsageSkeleton } from "./session-inspector-skeleton";
import type { InspectorUsage } from "./session-inspector-types";
import { SESSION_INSPECTOR_TILE } from "./session-inspector-tile";

function formatNumber(value?: number): string {
  if (typeof value !== "number" || !Number.isFinite(value)) return "—";
  return value.toLocaleString();
}

type UsageCost = ReturnType<typeof describeCost>;

/** The in/out/cache split, for the curious: one fold, closed by default. */
function UsageBreakdown({ usage }: { usage: InspectorUsage }) {
  return (
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
            className={SESSION_INSPECTOR_TILE}
            data-testid="session-inspector-usage-tokens-in"
            label="Tokens in"
            value={formatNumber(usage.tokensIn)}
          />
          <Metric
            size="compact"
            className={SESSION_INSPECTOR_TILE}
            data-testid="session-inspector-usage-tokens-out"
            label="Tokens out"
            value={formatNumber(usage.tokensOut)}
          />
          {usage.cacheReadTokens != null ? (
            <Metric
              size="compact"
              className={SESSION_INSPECTOR_TILE}
              label="Cache read"
              value={formatNumber(usage.cacheReadTokens)}
            />
          ) : null}
          {usage.cacheWriteTokens != null ? (
            <Metric
              size="compact"
              className={SESSION_INSPECTOR_TILE}
              label="Cache write"
              value={formatNumber(usage.cacheWriteTokens)}
            />
          ) : null}
        </MetricGrid>
      </CollapsibleContent>
    </Collapsible>
  );
}

/** What the agent reported: the cost and total tiles, then the closed token breakdown. */
function ReportedUsage({ usage, cost }: { usage: InspectorUsage; cost: UsageCost }) {
  return (
    <>
      <MetricGrid
        columns={2}
        className="grid-cols-2 gap-2"
        data-testid="session-inspector-usage-grid"
      >
        <Metric
          size="compact"
          className={SESSION_INSPECTOR_TILE}
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
          className={SESSION_INSPECTOR_TILE}
          data-testid="session-inspector-usage-total-tokens"
          label="Total tokens"
          value={formatNumber(usage.totalTokens)}
        />
      </MetricGrid>
      <UsageBreakdown usage={usage} />
    </>
  );
}

/** Usage while its first read is in flight, once reported, or when the agent has reported nothing. */
function UsageBody({
  usage,
  cost,
  loading,
}: {
  usage: InspectorUsage | null | undefined;
  cost: UsageCost;
  loading: boolean;
}) {
  if (usage != null && hasReportableUsage(usage, cost)) {
    return <ReportedUsage usage={usage} cost={cost} />;
  }
  if (loading) return <SessionInspectorUsageSkeleton />;
  return (
    <SessionInspectorEmpty
      data-testid="session-inspector-usage-empty"
      description="Token counts and cost land here once the agent reports its first turn."
      icon={Coins}
      title="No usage yet"
    />
  );
}

export function SessionInspectorUsageSection({
  usage,
  loading = false,
}: {
  usage: InspectorUsage | null | undefined;
  /** The first usage read is in flight: tiles, not the "No usage yet" empty state. */
  loading?: boolean;
}) {
  const cost = describeCost({
    status: usage?.costStatus,
    source: usage?.costSource,
    amount: usage?.costUsd,
    currency: usage?.costCurrency,
  });
  const turnCount = usage?.turnCount ?? 0;

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
      <UsageBody usage={usage} cost={cost} loading={loading} />
    </SessionInspectorSection>
  );
}
