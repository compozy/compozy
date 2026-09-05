import type { operations } from "@/generated/compozy-openapi";

import type { TerminalInfo } from "./lib/terminal-contract-schema";

export type { TerminalInfo } from "./lib/terminal-contract-schema";

type TerminalCreateRequest =
  operations["createTerminal"]["requestBody"]["content"]["application/json"];
type TerminalTicketRequest =
  operations["mintTerminalAttachTicket"]["requestBody"]["content"]["application/json"];
type TerminalSignalRequest =
  operations["signalTerminal"]["requestBody"]["content"]["application/json"];
type TerminalJournalResponse =
  operations["queryTerminalJournal"]["responses"][200]["content"]["application/json"];
type TerminalReadQuery = NonNullable<operations["readTerminal"]["parameters"]["query"]>;

export type TerminalActorKind = TerminalJournalResponse["entries"][number]["actor"]["kind"];
export type TerminalMode = TerminalInfo["mode"];
export type TerminalRunState = TerminalInfo["state"];
export type TerminalExitCause = NonNullable<TerminalInfo["exit"]>["cause"];
export type TerminalSignal = TerminalSignalRequest["signal"];
export type TerminalBoundRun = NonNullable<TerminalInfo["bound_run"]>;
export type TerminalExit = NonNullable<TerminalInfo["exit"]>;
export type TerminalCapabilities = TerminalInfo["capabilities"];

export interface TerminalExitNotice {
  cause: TerminalExitCause;
  code: number | null;
  signal: TerminalSignal | null;
}

export type TerminalAttachTicket =
  operations["mintTerminalAttachTicket"]["responses"][201]["content"]["application/json"];

export interface TerminalViewerIdentity {
  id: string;
  attachmentToken: string;
}

export type TerminalAttachMode = TerminalTicketRequest["mode"];
export type TerminalFlowMode = NonNullable<
  NonNullable<operations["streamTerminal"]["parameters"]["query"]>["flow"]
>;

export type CreateTerminalInput = Omit<TerminalCreateRequest, "client_id">;

export interface TerminalInputActorProjection {
  kind: TerminalActorKind;
  id: string;
}

export interface TerminalInputRequest {
  id: string;
  terminal_id: string;
  workspace_id?: string;
  profile_id: string;
  profile_name: string;
  prompt_excerpt: string;
  reason: string;
  redacted: boolean;
  requested_at: string;
  requester: TerminalInputActorProjection;
}

export type TerminalPendingInputRequest = TerminalInputRequest;

export interface TerminalResolvedInputRequest {
  id: string;
  terminal_id: string;
  workspace_id?: string;
  profile_id: string;
  profile_name: string;
  requester: TerminalInputActorProjection;
  outcome: TerminalInputOutcome;
  resolved_by: TerminalInputActorProjection;
  reason?: string;
  redacted: boolean;
  length: number;
  requested_at: string;
  resolved_at: string;
}

export interface TerminalInputRequestProjection {
  pending: TerminalPendingInputRequest[];
  resolved: TerminalResolvedInputRequest[];
}

export type TerminalInputOutcome = "answered" | "rejected" | "superseded" | "expired";

export type TerminalInputAnswerResult =
  operations["answerTerminalInputRequest"]["responses"][200]["content"]["application/json"];
export type TerminalInputRejectResult =
  operations["rejectTerminalInputRequest"]["responses"][200]["content"]["application/json"];

export type TerminalJournalEntry = TerminalJournalResponse["entries"][number];
export type TerminalDetectedBy = TerminalJournalEntry["detected_by"];
export type TerminalApproval = TerminalJournalEntry["approval"];
export type TerminalJournalPage = TerminalJournalResponse;

export interface TerminalJournalFilters {
  actor?: TerminalActorKind | null;
  since?: string | null;
  failed?: boolean;
  terminalId?: string | null;
  limit?: number;
}

export type TerminalRecording =
  operations["controlTerminalRecording"]["responses"][200]["content"]["application/json"]["recording"];

export type TerminalReadView = NonNullable<TerminalReadQuery["view"]>;
type GeneratedTerminalReadResult =
  operations["readTerminal"]["responses"][200]["content"]["application/json"];
export type TerminalReadResult = Omit<GeneratedTerminalReadResult, "seq"> & { seq: bigint };

export type TerminalScopeParams =
  | { profile: string; all_profiles?: never }
  | { profile?: never; all_profiles: true };

export type TerminalProfileScopeParams = Extract<TerminalScopeParams, { profile: string }>;

export type TerminalWaitUntil = "exit" | "idle" | "match";
export type TerminalWaitResult =
  operations["waitTerminal"]["responses"][200]["content"]["application/json"];

export interface TerminalScopeKey {
  workspaceId: string;
  profileKey: string;
}
