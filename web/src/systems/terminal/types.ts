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

export type TerminalLeaseState = TerminalInfo["lease"];
export type TerminalActorKind = NonNullable<TerminalInfo["controller"]>["kind"];
export type TerminalMode = TerminalInfo["mode"];
export type TerminalRunState = TerminalInfo["state"];
export type TerminalExitCause = NonNullable<TerminalInfo["exit"]>["cause"];
export type TerminalSignal = TerminalSignalRequest["signal"];
export type TerminalActor = NonNullable<TerminalInfo["controller"]>;
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

export type TerminalInputRequest =
  operations["listTerminalInputRequests"]["responses"][200]["content"]["application/json"]["requests"][number];

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
export type TerminalReadResult =
  operations["readTerminal"]["responses"][200]["content"]["application/json"];

export type TerminalScopeParams =
  | { profile: string; all_profiles?: never }
  | { profile?: never; all_profiles: true };

export type TerminalProfileScopeParams = Extract<TerminalScopeParams, { profile: string }>;

export interface TerminalScopeKey {
  workspaceId: string;
  profileKey: string;
}
