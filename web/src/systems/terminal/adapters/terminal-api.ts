/**
 * Terminal REST surface.
 *
 * The adapter goes through the shared runtime fetch and validates every JSON
 * envelope before exposing generated terminal contract types.
 */

import {
  clientID,
  computerApiBaseUrl,
  computerApiPath,
  daemonFetch,
  type ComputerId,
} from "@/lib/api-client";

import {
  terminalAttachTicketResponseSchema,
  terminalExitResponseSchema,
  terminalInputAnswerResponseSchema,
  terminalInputRejectResponseSchema,
  terminalInputRequestsResponseSchema,
  terminalJournalResponseSchema,
  terminalListResponseSchema,
  terminalReadResponseSchema,
  terminalRecordingResponseSchema,
  terminalResponseSchema,
  terminalSignalResponseSchema,
  terminalWaitResponseSchema,
} from "../lib/terminal-contract-schema";
import type {
  CreateTerminalInput,
  TerminalAttachMode,
  TerminalAttachTicket,
  TerminalExit,
  TerminalInfo,
  TerminalInputAnswerResult,
  TerminalInputRejectResult,
  TerminalInputRequestProjection,
  TerminalJournalFilters,
  TerminalJournalPage,
  TerminalProfileScopeParams,
  TerminalReadResult,
  TerminalReadView,
  TerminalRecording,
  TerminalScopeParams,
  TerminalSignal,
  TerminalViewerIdentity,
  TerminalWaitResult,
  TerminalWaitUntil,
} from "../types";
import {
  parseTerminalResponse,
  readTerminalBody,
  terminalResourceURL,
  terminalResponseError,
  terminalScopeQuery,
  terminalViewerHeaders,
  terminalWorkspaceRoot,
  withTerminalQuery,
} from "./terminal-api-http";

export { TerminalApiError, TerminalProtocolError, terminalScopeQuery } from "./terminal-api-http";

export async function fetchTerminals(
  workspaceId: string,
  scope: TerminalScopeParams,
  signal?: AbortSignal,
  computerId?: ComputerId
): Promise<TerminalInfo[]> {
  const payload = await parseTerminalResponse(
    withTerminalQuery(terminalWorkspaceRoot(workspaceId, computerId), terminalScopeQuery(scope)),
    terminalListResponseSchema,
    { method: "GET", signal }
  );
  return payload.terminals;
}

/**
 * Every project's terminals plus User's (operator, local listener only),
 * narrowed by the Project filter. Each row carries its own `workspace_id`.
 */
export async function fetchTerminalsAggregate(
  workspaceIds: readonly string[],
  scope: TerminalScopeParams,
  signal?: AbortSignal,
  computerId?: ComputerId
): Promise<TerminalInfo[]> {
  const query = terminalScopeQuery(scope);
  query.set("all_workspaces", "true");
  for (const id of workspaceIds) query.append("workspace_ids", id);
  const payload = await parseTerminalResponse(
    withTerminalQuery(`${computerApiBaseUrl(computerId)}/api/terminals`, query),
    terminalListResponseSchema,
    { method: "GET", signal }
  );
  return payload.terminals;
}

export async function fetchTerminal(
  workspaceId: string,
  terminalId: string,
  scope: TerminalProfileScopeParams,
  signal?: AbortSignal,
  computerId?: ComputerId
): Promise<TerminalInfo> {
  const url = terminalResourceURL(workspaceId, terminalId, "", computerId);
  const payload = await parseTerminalResponse(
    withTerminalQuery(url, terminalScopeQuery({ profile: scope.profile })),
    terminalResponseSchema,
    { method: "GET", signal }
  );
  return payload.terminal;
}

/** Creates a terminal in the requested data scope using proof from the viewer's registered desktop. */
export async function createTerminal(
  workspaceId: string,
  input: CreateTerminalInput,
  scope: TerminalProfileScopeParams,
  viewer?: TerminalViewerIdentity | null,
  signal?: AbortSignal,
  computerId?: ComputerId
): Promise<TerminalInfo> {
  const payload = await parseTerminalResponse(
    withTerminalQuery(
      terminalWorkspaceRoot(workspaceId, computerId),
      terminalScopeQuery({ profile: scope.profile })
    ),
    terminalResponseSchema,
    {
      method: "POST",
      body: JSON.stringify({ ...input, ...(viewer ? { client_id: viewer.id } : {}) }),
      headers: viewer ? terminalViewerHeaders(viewer) : undefined,
      signal,
    }
  );
  return payload.terminal;
}

export async function closeTerminal(
  workspaceId: string,
  terminalId: string,
  scope: TerminalProfileScopeParams,
  terminalSignal?: TerminalSignal,
  abortSignal?: AbortSignal,
  computerId?: ComputerId
): Promise<TerminalExit | null> {
  const url = terminalResourceURL(workspaceId, terminalId, "", computerId);
  const payload = await parseTerminalResponse(
    withTerminalQuery(url, terminalScopeQuery({ profile: scope.profile })),
    terminalExitResponseSchema,
    {
      method: "DELETE",
      body: terminalSignal ? JSON.stringify({ signal: terminalSignal }) : undefined,
      signal: abortSignal,
    }
  );
  return payload.exit;
}

/**
 * Mints a single-use attach pass. Every upgrade attempt gets its own: a reused
 * pass is `ticket_invalid`, which is why the protocol client mints per attempt
 * rather than caching one.
 */
export async function mintTerminalAttachTicket(
  workspaceId: string,
  terminalId: string,
  mode: TerminalAttachMode,
  scope: TerminalProfileScopeParams,
  viewer?: TerminalViewerIdentity | null,
  signal?: AbortSignal,
  computerId?: ComputerId
): Promise<TerminalAttachTicket> {
  const url = terminalResourceURL(workspaceId, terminalId, "/attach-ticket", computerId);
  return parseTerminalResponse(
    withTerminalQuery(url, terminalScopeQuery({ profile: scope.profile })),
    terminalAttachTicketResponseSchema,
    {
      method: "POST",
      body: JSON.stringify({ mode, ...(viewer ? { client_id: viewer.id } : {}) }),
      headers: viewer ? terminalViewerHeaders(viewer) : undefined,
      signal,
    }
  );
}

export interface TerminalReadParams {
  view: TerminalReadView;
  maxBytes?: number;
  sinceSeq?: bigint;
  from?: number;
  to?: number;
  grep?: string;
}

export async function readTerminal(
  workspaceId: string,
  terminalId: string,
  params: TerminalReadParams,
  scope: TerminalProfileScopeParams,
  signal?: AbortSignal,
  computerId?: ComputerId
): Promise<TerminalReadResult> {
  const query = terminalScopeQuery({ profile: scope.profile });
  query.set("view", params.view);
  if (params.maxBytes !== undefined) query.set("max_bytes", String(params.maxBytes));
  if (params.sinceSeq !== undefined) query.set("since_seq", String(params.sinceSeq));
  if (params.from !== undefined) query.set("from", String(params.from));
  if (params.to !== undefined) query.set("to", String(params.to));
  if (params.grep) query.set("grep", params.grep);
  const url = terminalResourceURL(workspaceId, terminalId, "/read", computerId);
  return parseTerminalResponse(withTerminalQuery(url, query), terminalReadResponseSchema, {
    method: "GET",
    signal,
  });
}

export async function signalTerminal(
  workspaceId: string,
  terminalId: string,
  signal: TerminalSignal,
  scope: TerminalProfileScopeParams,
  abortSignal?: AbortSignal,
  computerId?: ComputerId
): Promise<void> {
  const url = terminalResourceURL(workspaceId, terminalId, "/signal", computerId);
  await parseTerminalResponse(
    withTerminalQuery(url, terminalScopeQuery({ profile: scope.profile })),
    terminalSignalResponseSchema,
    { method: "POST", body: JSON.stringify({ signal }), signal: abortSignal }
  );
}

export async function waitTerminal(
  workspaceId: string,
  terminalId: string,
  condition: { until: TerminalWaitUntil; pattern?: string; timeout_ms?: number },
  scope: TerminalProfileScopeParams,
  abortSignal?: AbortSignal,
  computerId?: ComputerId
): Promise<TerminalWaitResult> {
  const url = terminalResourceURL(workspaceId, terminalId, "/wait", computerId);
  return parseTerminalResponse(
    withTerminalQuery(url, terminalScopeQuery({ profile: scope.profile })),
    terminalWaitResponseSchema,
    { method: "POST", body: JSON.stringify(condition), signal: abortSignal }
  );
}

export async function fetchTerminalInputRequestProjection(
  workspaceId: string,
  scope: TerminalScopeParams,
  terminalId?: string,
  signal?: AbortSignal,
  computerId?: ComputerId
): Promise<TerminalInputRequestProjection> {
  const query = terminalScopeQuery(scope);
  if (terminalId) query.set("terminal_id", terminalId);
  const payload = await parseTerminalResponse(
    withTerminalQuery(`${terminalWorkspaceRoot(workspaceId, computerId)}/input-requests`, query),
    terminalInputRequestsResponseSchema,
    { method: "GET", signal }
  );
  return payload;
}

function inputRequestRoot(
  workspaceId: string,
  terminalId: string,
  requestId: string,
  computerId?: ComputerId
): string {
  return terminalResourceURL(
    workspaceId,
    terminalId,
    `/input-requests/${encodeURIComponent(requestId)}`,
    computerId
  );
}

/**
 * Delivers an answer. Redaction is the daemon's call — the request's own
 * `redacted` flag may only raise it — so nothing here decides it.
 */
export async function answerTerminalInputRequest(
  workspaceId: string,
  terminalId: string,
  requestId: string,
  input: string,
  scope: TerminalProfileScopeParams,
  abortSignal?: AbortSignal,
  computerId?: ComputerId
): Promise<TerminalInputAnswerResult> {
  return parseTerminalResponse(
    withTerminalQuery(
      `${inputRequestRoot(workspaceId, terminalId, requestId, computerId)}/answer`,
      terminalScopeQuery({ profile: scope.profile })
    ),
    terminalInputAnswerResponseSchema,
    { method: "POST", body: JSON.stringify({ input }), signal: abortSignal }
  );
}

export async function rejectTerminalInputRequest(
  workspaceId: string,
  terminalId: string,
  requestId: string,
  reason: string,
  scope: TerminalProfileScopeParams,
  abortSignal?: AbortSignal,
  computerId?: ComputerId
): Promise<TerminalInputRejectResult> {
  return parseTerminalResponse(
    withTerminalQuery(
      `${inputRequestRoot(workspaceId, terminalId, requestId, computerId)}/reject`,
      terminalScopeQuery({ profile: scope.profile })
    ),
    terminalInputRejectResponseSchema,
    { method: "POST", body: JSON.stringify({ reason }), signal: abortSignal }
  );
}

export async function controlTerminalRecording(
  workspaceId: string,
  terminalId: string,
  action: "start" | "stop",
  scope: TerminalProfileScopeParams,
  abortSignal?: AbortSignal,
  computerId?: ComputerId
): Promise<TerminalRecording> {
  const url = terminalResourceURL(workspaceId, terminalId, "/recording", computerId);
  const payload = await parseTerminalResponse(
    withTerminalQuery(url, terminalScopeQuery({ profile: scope.profile })),
    terminalRecordingResponseSchema,
    { method: "POST", body: JSON.stringify({ action }), signal: abortSignal }
  );
  return payload.recording;
}

export async function fetchTerminalJournal(
  workspaceId: string,
  filters: TerminalJournalFilters,
  scope: TerminalScopeParams,
  cursor?: string | null,
  signal?: AbortSignal,
  computerId?: ComputerId
): Promise<TerminalJournalPage> {
  const query = terminalScopeQuery(scope);
  if (filters.actor) query.set("actor", filters.actor);
  if (filters.since) query.set("since", filters.since);
  if (filters.failed) query.set("failed", "true");
  if (filters.terminalId) query.set("terminal_id", filters.terminalId);
  if (filters.limit !== undefined) query.set("limit", String(filters.limit));
  if (cursor) query.set("cursor", cursor);
  return parseTerminalResponse(
    withTerminalQuery(`${terminalWorkspaceRoot(workspaceId, computerId)}/journal`, query),
    terminalJournalResponseSchema,
    { method: "GET", signal }
  );
}

/** The recording artifact, as asciicast v2 text. */
export async function fetchTerminalRecording(
  workspaceId: string,
  recordingId: string,
  scope: TerminalProfileScopeParams,
  signal?: AbortSignal,
  computerId?: ComputerId
): Promise<string> {
  const url = withTerminalQuery(
    `${terminalWorkspaceRoot(workspaceId, computerId)}/recordings/${encodeURIComponent(recordingId)}`,
    terminalScopeQuery({ profile: scope.profile })
  );
  const response = await daemonFetch(url, {
    method: "GET",
    signal,
    headers: { "X-Compozy-Client-ID": clientID },
  });
  if (!response.ok) {
    const body = await readTerminalBody(response);
    throw terminalResponseError(response, body);
  }
  return response.text();
}

/** The stream upgrade URL. The ticket binding carries the profile, not a query. */
export function terminalStreamPath(
  workspaceId: string,
  terminalId: string,
  params: {
    ticket: string;
    mode: TerminalAttachMode;
    cols?: number;
    rows?: number;
    afterSeq?: bigint;
    flow: "drop" | "ack";
  },
  computerId?: ComputerId
): string {
  const query = new URLSearchParams({
    ticket: params.ticket,
    mode: params.mode,
    flow: params.flow,
  });
  if (params.cols !== undefined) query.set("cols", String(params.cols));
  if (params.rows !== undefined) query.set("rows", String(params.rows));
  if (params.afterSeq !== undefined) query.set("after_seq", String(params.afterSeq));
  return computerApiPath(
    computerId,
    `/api/workspaces/${encodeURIComponent(workspaceId)}/terminals/${encodeURIComponent(terminalId)}/stream?${query.toString()}`
  );
}
