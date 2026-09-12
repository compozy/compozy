import type { CostDisplay } from "@/lib/cost-provenance";

/** Token/turn fields the Usage-presence predicate reads (structural subset of InspectorUsage). */
interface UsagePresenceFields {
  tokensIn?: number;
  cacheReadTokens?: number;
  cacheWriteTokens?: number;
  tokensOut?: number;
  totalTokens?: number;
  turnCount?: number;
}

/**
 * The Usage panel opens for any reported token counter, an authoritative cost
 * status (`cost.hasCost`, owned by describeCost), or a positive turn count. A
 * statusless amount alone never opens it: describeCost reports `hasCost: false`
 * for it, so this predicate excludes it by construction.
 */
export function hasReportableUsage(usage: UsagePresenceFields, cost: CostDisplay): boolean {
  return (
    usage.tokensIn !== undefined ||
    usage.cacheReadTokens !== undefined ||
    usage.cacheWriteTokens !== undefined ||
    usage.tokensOut !== undefined ||
    usage.totalTokens !== undefined ||
    cost.hasCost ||
    (usage.turnCount ?? 0) > 0
  );
}
