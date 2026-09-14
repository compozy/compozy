import type { CostSource, CostStatus } from "@/lib/cost-provenance";

export interface InspectorUsage {
  tokensIn?: number;
  cacheReadTokens?: number;
  cacheWriteTokens?: number;
  tokensOut?: number;
  totalTokens?: number;
  costUsd?: number;
  costCurrency?: string;
  costStatus?: CostStatus;
  costSource?: CostSource;
  turnCount?: number;
}
