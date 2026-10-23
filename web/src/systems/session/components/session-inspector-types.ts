import type { CostSource, CostStatus } from "@/lib/cost-provenance";
import type { SessionUsagePayload } from "../types";

export interface InspectorUsage {
  workTime?: SessionUsagePayload["work_time"];
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
