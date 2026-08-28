/**
 * Terminal REST surface.
 *
 * The adapter goes through the shared runtime fetch and validates every JSON
 * envelope before exposing generated terminal contract types.
 */

import { apiBaseUrl, runtimeFetch } from "@/lib/api-client";
import type { ZodType } from "zod";

import {
  terminalAttachTicketResponseSchema,
  terminalErrorEnvelopeSchema,
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
  terminalErrorCodeSchema,
  type TerminalErrorCode,
  type TerminalErrorDetails,
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
  TerminalReadResult,
  TerminalReadView,
  TerminalRecording,
  TerminalProfileScopeParams,
  TerminalScopeParams,
  TerminalSignal,
  TerminalViewerIdentity,
} from "../types";

/**
 * Carries the daemon's machine code beside the message.
 *
 * Terminal domain refusals use the frozen code set. Transport failures keep
 * the same envelope with truthful generic codes such as `invalid_request` or
 * `service_unavailable`, so callers may branch only when `domainCode` exists.
 */
export class TerminalApiError extends Error {
  public readonly details: Readonly<TerminalErrorDetails> | undefined;
  public readonly domainCode: TerminalErrorCode | undefined;

  constructor(
    message: string,
    public readonly status: number,
    public readonly code: string,
    details?: Readonly<TerminalErrorDetails>
  ) {
    super(message);
    this.name = "TerminalApiError";
    this.details = details ? Object.freeze({ ...details }) : undefined;
    const parsedDomainCode = terminalErrorCodeSchema.safeParse(code);
    this.domainCode = parsedDomainCode.success ? parsedDomainCode.data : undefined;
  }
}

const TERMINAL_PROTOCOL_ERROR_MESSAGE = "The daemon returned an invalid terminal response.";

export class TerminalProtocolError extends Error {
  constructor(public readonly status: number) {
    super(TERMINAL_PROTOCOL_ERROR_MESSAGE);
    this.name = "TerminalProtocolError";
  }
}

function workspaceRoot(workspaceId: string): string {
  return `${apiBaseUrl}/api/workspaces/${encodeURIComponent(workspaceId)}/terminals`;
}

function terminalURL(workspaceId: string, terminalId: string, suffix = ""): string {
  return `${workspaceRoot(workspaceId)}/${encodeURIComponent(terminalId)}${suffix}`;
}

/**
 * Builds the query the daemon expects. `profile` and `all_profiles` are
 * mutually exclusive — sending both is `profile_selection_conflict` — so the
 * aggregate selector wins only where a caller explicitly asked for it.
 */
export function terminalScopeQuery(scope: TerminalScopeParams): URLSearchParams {
  const query = new URLSearchParams();
  if (scope.all_profiles) {
    query.set("all_profiles", "true");
  } else if (scope.profile) {
    query.set("profile", scope.profile);
  }
  return query;
}

function withQuery(url: string, query: URLSearchParams): string {
  const serialized = query.toString();
  return serialized === "" ? url : `${url}?${serialized}`;
}

async function parseTerminalResponse<Result>(
  url: string,
  schema: ZodType<Result>,
  init?: RequestInit
): Promise<Result> {
  const response = await runtimeFetch(url, {
    ...init,
    headers: { "Content-Type": "application/json", ...init?.headers },
  });
  const body = await readBody(response);
  if (!response.ok) {
    throw terminalResponseError(response, body);
  }
  const parsed = schema.safeParse(body);
  if (!parsed.success) throw new TerminalProtocolError(response.status);
  return parsed.data;
}

function terminalResponseError(response: Response, body: unknown): TerminalApiError {
  const parsed = terminalErrorEnvelopeSchema.safeParse(body);
  if (!parsed.success) throw new TerminalProtocolError(response.status);
  return new TerminalApiError(
    parsed.data.error.message,
    response.status,
    parsed.data.error.code,
    parsed.data.error.details
  );
}

async function readBody(response: Response): Promise<unknown> {
  const text = await response.text();
  if (text.trim() === "") return undefined;
  try {
    return JSON.parse(text) as unknown;
  } catch {
    throw new TerminalProtocolError(response.status);
  }
}

export async function fetchTerminals(
  workspaceId: string,
  scope: TerminalScopeParams,
  signal?: AbortSignal
): Promise<TerminalInfo[]> {
  const payload = await parseTerminalResponse(
    withQuery(workspaceRoot(workspaceId), terminalScopeQuery(scope)),
    terminalListResponseSchema,
    { method: "GET", signal }
  );
  return payload.terminals;
}

export async function fetchTerminal(
  workspaceId: string,
  terminalId: string,
  scope: TerminalProfileScopeParams,
  signal?: AbortSignal
): Promise<TerminalInfo> {
  const url = terminalURL(workspaceId, terminalId);
  const payload = await parseTerminalResponse(
    withQuery(url, terminalScopeQuery({ profile: scope.profile })),
    terminalResponseSchema,
    { method: "GET", signal }
  );
  return payload.terminal;
}

export async function createTerminal(
  workspaceId: string,
  input: CreateTerminalInput,
  scope: TerminalProfileScopeParams,
  viewer: TerminalViewerIdentity,
  signal?: AbortSignal
): Promise<TerminalInfo> {
  const payload = await parseTerminalResponse(
    withQuery(workspaceRoot(workspaceId), terminalScopeQuery({ profile: scope.profile })),
    terminalResponseSchema,
    {
      method: "POST",
      body: JSON.stringify({ ...input, client_id: viewer.id }),
      headers: { "X-Compozy-Client-Token": viewer.attachmentToken },
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
  abortSignal?: AbortSignal
): Promise<TerminalExit | null> {
  const url = terminalURL(workspaceId, terminalId);
  const payload = await parseTerminalResponse(
    withQuery(url, terminalScopeQuery({ profile: scope.profile })),
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
  signal?: AbortSignal
): Promise<TerminalAttachTicket> {
  const url = terminalURL(workspaceId, terminalId, "/attach-ticket");
  return parseTerminalResponse(
    withQuery(url, terminalScopeQuery({ profile: scope.profile })),
    terminalAttachTicketResponseSchema,
    {
      method: "POST",
      body: JSON.stringify({ mode, ...(viewer ? { client_id: viewer.id } : {}) }),
      headers: viewer ? { "X-Compozy-Client-Token": viewer.attachmentToken } : undefined,
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
  signal?: AbortSignal
): Promise<TerminalReadResult> {
  const query = terminalScopeQuery({ profile: scope.profile });
  query.set("view", params.view);
  if (params.maxBytes !== undefined) query.set("max_bytes", String(params.maxBytes));
  if (params.sinceSeq !== undefined) query.set("since_seq", String(params.sinceSeq));
  if (params.from !== undefined) query.set("from", String(params.from));
  if (params.to !== undefined) query.set("to", String(params.to));
  if (params.grep) query.set("grep", params.grep);
  const url = terminalURL(workspaceId, terminalId, "/read");
  return parseTerminalResponse(withQuery(url, query), terminalReadResponseSchema, {
    method: "GET",
    signal,
  });
}

export async function signalTerminal(
  workspaceId: string,
  terminalId: string,
  signal: TerminalSignal,
  scope: TerminalProfileScopeParams,
  abortSignal?: AbortSignal
): Promise<void> {
  const url = terminalURL(workspaceId, terminalId, "/signal");
  await parseTerminalResponse(
    withQuery(url, terminalScopeQuery({ profile: scope.profile })),
    terminalSignalResponseSchema,
    { method: "POST", body: JSON.stringify({ signal }), signal: abortSignal }
  );
}

export async function fetchTerminalInputRequestProjection(
  workspaceId: string,
  scope: TerminalScopeParams,
  terminalId?: string,
  signal?: AbortSignal
): Promise<TerminalInputRequestProjection> {
  const query = terminalScopeQuery(scope);
  if (terminalId) query.set("terminal_id", terminalId);
  const payload = await parseTerminalResponse(
    withQuery(`${workspaceRoot(workspaceId)}/input-requests`, query),
    terminalInputRequestsResponseSchema,
    { method: "GET", signal }
  );
  return payload;
}

function inputRequestRoot(workspaceId: string, terminalId: string, requestId: string): string {
  return terminalURL(workspaceId, terminalId, `/input-requests/${encodeURIComponent(requestId)}`);
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
  abortSignal?: AbortSignal
): Promise<TerminalInputAnswerResult> {
  return parseTerminalResponse(
    withQuery(
      `${inputRequestRoot(workspaceId, terminalId, requestId)}/answer`,
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
  abortSignal?: AbortSignal
): Promise<TerminalInputRejectResult> {
  return parseTerminalResponse(
    withQuery(
      `${inputRequestRoot(workspaceId, terminalId, requestId)}/reject`,
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
  abortSignal?: AbortSignal
): Promise<TerminalRecording> {
  const url = terminalURL(workspaceId, terminalId, "/recording");
  const payload = await parseTerminalResponse(
    withQuery(url, terminalScopeQuery({ profile: scope.profile })),
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
  signal?: AbortSignal
): Promise<TerminalJournalPage> {
  const query = terminalScopeQuery(scope);
  if (filters.actor) query.set("actor", filters.actor);
  if (filters.since) query.set("since", filters.since);
  if (filters.failed) query.set("failed", "true");
  if (filters.terminalId) query.set("terminal_id", filters.terminalId);
  if (filters.limit !== undefined) query.set("limit", String(filters.limit));
  if (cursor) query.set("cursor", cursor);
  return parseTerminalResponse(
    withQuery(`${workspaceRoot(workspaceId)}/journal`, query),
    terminalJournalResponseSchema,
    { method: "GET", signal }
  );
}

/** The recording artifact, as asciicast v2 text. */
export async function fetchTerminalRecording(
  workspaceId: string,
  recordingId: string,
  scope: TerminalProfileScopeParams,
  signal?: AbortSignal
): Promise<string> {
  const url = withQuery(
    `${workspaceRoot(workspaceId)}/recordings/${encodeURIComponent(recordingId)}`,
    terminalScopeQuery({ profile: scope.profile })
  );
  const response = await runtimeFetch(url, { method: "GET", signal });
  if (!response.ok) {
    const body = await readBody(response);
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
  }
): string {
  const query = new URLSearchParams({
    ticket: params.ticket,
    mode: params.mode,
    flow: params.flow,
  });
  if (params.cols !== undefined) query.set("cols", String(params.cols));
  if (params.rows !== undefined) query.set("rows", String(params.rows));
  if (params.afterSeq !== undefined) query.set("after_seq", String(params.afterSeq));
  return `/api/workspaces/${encodeURIComponent(workspaceId)}/terminals/${encodeURIComponent(terminalId)}/stream?${query.toString()}`;
}
