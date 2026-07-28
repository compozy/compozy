import type { CostSource, CostStatus } from "@/lib/cost-provenance";
import type { SessionLedgerEvent, SessionLedgerMeta } from "../types";

export interface InspectorUsage {
  tokensIn?: number;
  tokensOut?: number;
  totalTokens?: number;
  costUsd?: number;
  costCurrency?: string;
  costStatus?: CostStatus;
  costSource?: CostSource;
  turnCount?: number;
}

export interface InspectorSessionLedger {
  meta: SessionLedgerMeta;
  events: readonly SessionLedgerEvent[];
}

export interface InspectorMemoryState {
  ledger?: InspectorSessionLedger | null;
  isLoading?: boolean;
  error?: Error | null;
}
