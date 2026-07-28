import type { OperationQuery, OperationResponse } from "@/lib/api-contract";

export type HomeOverview = OperationResponse<"getObserveOverview", 200>["overview"];
export type HomeOverviewWireFilter = OperationQuery<"getObserveOverview">;

export interface HomeOverviewFilter {
  workspace?: string;
  usageWindow?: HomeUsageWindow;
}
export type HomeAttention = HomeOverview["attention"];
export type HomeAttentionItem = HomeAttention["items"][number];
export type HomeOutcomeDay = HomeOverview["outcomes"]["days"][number];
export type HomeUsageDay = HomeOverview["usage"]["days"][number];
export type HomeAgentShare = HomeOverview["usage"]["agent_share"][number];
export type HomePulseBucket = HomeOverview["pulse"]["buckets"][number];

export type HomeActivityEvent = OperationResponse<"listLogs", 200>["events"][number];
export type HomeActivityFilter = OperationQuery<"listLogs">;

export type HomeUsageWindow = 7 | 30 | 90;

export type HomeSurfaceStatus = "loading" | "error" | "ready";

// `partial` is only produced by the multi-source working-now surface (sessions +
// task runs): some cards loaded while another source failed, so the set is shown
// but flagged incomplete rather than passed off as whole. It stays off the shared
// `HomeSurfaceStatus` so single-source consumers (overview → DataSurface) keep a
// closed status set.
export type HomeWorkingNowStatus = HomeSurfaceStatus | "partial";

export interface HomeRunCardModel {
  key: string;
  kind: "session" | "task_run";
  agentName: string;
  title: string;
  subtitle: string;
  /** Elapsed seconds at snapshot time; the card ticks forward from `baseAtMs`. */
  elapsedBaseSeconds: number;
  baseAtMs: number;
  sessionLink?: { agentName: string; sessionId: string };
  runLink?: { taskId: string; runId: string };
}

export interface HomeWorkingNowModel {
  /** The deduplicated live work items actually loaded (session + unique run cards). */
  cards: HomeRunCardModel[];
  /** Count of the loaded, deduplicated live set — equals `cards.length`, never an aggregate. */
  total: number;
  /** Loaded session cards. */
  sessionCount: number;
  /** Loaded task-run cards not bound to a loaded session. */
  runCount: number;
  status: HomeWorkingNowStatus;
  /** Message from the first failed source when a read errored; drives the error/partial copy. */
  errorMessage: string | null;
}

export const HOME_USAGE_WINDOWS: readonly HomeUsageWindow[] = [7, 30, 90];

export function normalizeHomeUsageWindow(value: number): HomeUsageWindow {
  return HOME_USAGE_WINDOWS.find(window => window === value) ?? 30;
}
