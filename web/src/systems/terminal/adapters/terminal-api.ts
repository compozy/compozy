/**
 * Terminal REST surface.
 *
 * The generated OpenAPI module does not describe these routes yet — they are
 * registered in the public-activation tranche — so this adapter goes through
 * the shared runtime fetch with the hand-written types in `../types`.
 */

import { apiBaseUrl, runtimeFetch } from "@/lib/api-client";

import type {
  CreateTerminalInput,
  TerminalAttachMode,
  TerminalAttachTicket,
  TerminalExit,
  TerminalInfo,
  TerminalInputAnswerResult,
  TerminalInputRejectResult,
  TerminalInputRequest,
  TerminalJournalFilters,
  TerminalJournalPage,
  TerminalReadResult,
  TerminalReadView,
  TerminalRecording,
  TerminalScopeParams,
  TerminalSignal,
} from "../types";

/**
 * Carries the daemon's machine code beside the message.
 *
 * Every terminal refusal is typed (`terminal_limit_reached`, `ticket_expired`,
 * `journal_unavailable`, …) and the surfaces branch on the code, so the prose
 * stays free to change without breaking behaviour.
 */
export class TerminalApiError extends Error {
  constructor(
    message: string,
    public readonly status: number,
    public readonly code: string
  ) {
    super(message);
    this.name = "TerminalApiError";
  }
}

interface TerminalErrorEnvelope {
  error?: { code?: string; message?: string };
}

function workspaceRoot(workspaceId: string): string {
  return `${apiBaseUrl}/api/workspaces/${encodeURIComponent(workspaceId)}/terminals`;
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

async function terminalRequest<T>(url: string, fallback: string, init?: RequestInit): Promise<T> {
  const response = await runtimeFetch(url, {
    ...init,
    headers: { "Content-Type": "application/json", ...init?.headers },
  });
  const body = await readBody(response);
  if (!response.ok) {
    const envelope = (body ?? {}) as TerminalErrorEnvelope;
    const message = envelope.error?.message?.trim();
    throw new TerminalApiError(
      message && message !== "" ? message : `${fallback}: ${response.status}`,
      response.status,
      envelope.error?.code?.trim() ?? ""
    );
  }
  return body as T;
}

async function readBody(response: Response): Promise<unknown> {
  const text = await response.text();
  if (text.trim() === "") return undefined;
  try {
    return JSON.parse(text) as unknown;
  } catch {
    throw new TerminalApiError("The daemon returned an unreadable response.", response.status, "");
  }
}

export async function fetchTerminals(
  workspaceId: string,
  scope: TerminalScopeParams,
  signal?: AbortSignal
): Promise<TerminalInfo[]> {
  const payload = await terminalRequest<{ terminals: TerminalInfo[] }>(
    withQuery(workspaceRoot(workspaceId), terminalScopeQuery(scope)),
    "Failed to load terminals",
    { method: "GET", signal }
  );
  return payload.terminals;
}

export async function fetchTerminal(
  workspaceId: string,
  terminalId: string,
  scope: TerminalScopeParams,
  signal?: AbortSignal
): Promise<TerminalInfo> {
  const url = `${workspaceRoot(workspaceId)}/${encodeURIComponent(terminalId)}`;
  const payload = await terminalRequest<{ terminal: TerminalInfo }>(
    withQuery(url, terminalScopeQuery({ profile: scope.profile })),
    "Failed to load the terminal",
    { method: "GET", signal }
  );
  return payload.terminal;
}

export async function createTerminal(
  workspaceId: string,
  input: CreateTerminalInput,
  scope: TerminalScopeParams,
  signal?: AbortSignal
): Promise<TerminalInfo> {
  const payload = await terminalRequest<{ terminal: TerminalInfo }>(
    withQuery(workspaceRoot(workspaceId), terminalScopeQuery({ profile: scope.profile })),
    "Failed to open a terminal",
    { method: "POST", body: JSON.stringify(input), signal }
  );
  return payload.terminal;
}

export async function closeTerminal(
  workspaceId: string,
  terminalId: string,
  scope: TerminalScopeParams,
  signal?: TerminalSignal,
  abortSignal?: AbortSignal
): Promise<TerminalExit | null> {
  const url = `${workspaceRoot(workspaceId)}/${encodeURIComponent(terminalId)}`;
  const payload = await terminalRequest<{ exit: TerminalExit | null }>(
    withQuery(url, terminalScopeQuery({ profile: scope.profile })),
    "Failed to close the terminal",
    {
      method: "DELETE",
      body: signal ? JSON.stringify({ signal }) : undefined,
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
  scope: TerminalScopeParams,
  signal?: AbortSignal
): Promise<TerminalAttachTicket> {
  const url = `${workspaceRoot(workspaceId)}/${encodeURIComponent(terminalId)}/attach-ticket`;
  return terminalRequest<TerminalAttachTicket>(
    withQuery(url, terminalScopeQuery({ profile: scope.profile })),
    "Failed to open a connection pass",
    { method: "POST", body: JSON.stringify({ mode }), signal }
  );
}

export interface TerminalReadParams {
  view: TerminalReadView;
  maxBytes?: number;
  sinceSeq?: number;
  from?: number;
  to?: number;
  grep?: string;
}

export async function readTerminal(
  workspaceId: string,
  terminalId: string,
  params: TerminalReadParams,
  scope: TerminalScopeParams,
  signal?: AbortSignal
): Promise<TerminalReadResult> {
  const query = terminalScopeQuery({ profile: scope.profile });
  query.set("view", params.view);
  if (params.maxBytes !== undefined) query.set("max_bytes", String(params.maxBytes));
  if (params.sinceSeq !== undefined) query.set("since_seq", String(params.sinceSeq));
  if (params.from !== undefined) query.set("from", String(params.from));
  if (params.to !== undefined) query.set("to", String(params.to));
  if (params.grep) query.set("grep", params.grep);
  const url = `${workspaceRoot(workspaceId)}/${encodeURIComponent(terminalId)}/read`;
  return terminalRequest<TerminalReadResult>(withQuery(url, query), "Failed to read the terminal", {
    method: "GET",
    signal,
  });
}

export async function signalTerminal(
  workspaceId: string,
  terminalId: string,
  signal: TerminalSignal,
  scope: TerminalScopeParams,
  abortSignal?: AbortSignal
): Promise<void> {
  const url = `${workspaceRoot(workspaceId)}/${encodeURIComponent(terminalId)}/signal`;
  await terminalRequest<{ delivered: boolean }>(
    withQuery(url, terminalScopeQuery({ profile: scope.profile })),
    "Failed to deliver the signal",
    { method: "POST", body: JSON.stringify({ signal }), signal: abortSignal }
  );
}

export async function fetchTerminalInputRequests(
  workspaceId: string,
  scope: TerminalScopeParams,
  terminalId?: string,
  signal?: AbortSignal
): Promise<TerminalInputRequest[]> {
  const query = terminalScopeQuery(scope);
  if (terminalId) query.set("terminal_id", terminalId);
  const payload = await terminalRequest<{ requests: TerminalInputRequest[] }>(
    withQuery(`${workspaceRoot(workspaceId)}/input-requests`, query),
    "Failed to load pending questions",
    { method: "GET", signal }
  );
  return payload.requests;
}

function inputRequestRoot(workspaceId: string, terminalId: string, requestId: string): string {
  return `${workspaceRoot(workspaceId)}/${encodeURIComponent(terminalId)}/input-requests/${encodeURIComponent(requestId)}`;
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
  scope: TerminalScopeParams,
  abortSignal?: AbortSignal
): Promise<TerminalInputAnswerResult> {
  return terminalRequest<TerminalInputAnswerResult>(
    withQuery(
      `${inputRequestRoot(workspaceId, terminalId, requestId)}/answer`,
      terminalScopeQuery({ profile: scope.profile })
    ),
    "Failed to send the answer",
    { method: "POST", body: JSON.stringify({ input }), signal: abortSignal }
  );
}

export async function rejectTerminalInputRequest(
  workspaceId: string,
  terminalId: string,
  requestId: string,
  reason: string,
  scope: TerminalScopeParams,
  abortSignal?: AbortSignal
): Promise<TerminalInputRejectResult> {
  return terminalRequest<TerminalInputRejectResult>(
    withQuery(
      `${inputRequestRoot(workspaceId, terminalId, requestId)}/reject`,
      terminalScopeQuery({ profile: scope.profile })
    ),
    "Failed to decline the question",
    { method: "POST", body: JSON.stringify({ reason }), signal: abortSignal }
  );
}

export async function controlTerminalRecording(
  workspaceId: string,
  terminalId: string,
  action: "start" | "stop",
  scope: TerminalScopeParams,
  abortSignal?: AbortSignal
): Promise<TerminalRecording> {
  const url = `${workspaceRoot(workspaceId)}/${encodeURIComponent(terminalId)}/recording`;
  const payload = await terminalRequest<{ recording: TerminalRecording }>(
    withQuery(url, terminalScopeQuery({ profile: scope.profile })),
    "Failed to change the recording",
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
  const page = await terminalRequest<TerminalJournalPage>(
    withQuery(`${workspaceRoot(workspaceId)}/journal`, query),
    "Failed to load the journal",
    { method: "GET", signal }
  );
  return { entries: page.entries ?? [], next: page.next ?? null };
}

/** The recording artifact, as asciicast v2 text. */
export async function fetchTerminalRecording(
  workspaceId: string,
  recordingId: string,
  scope: TerminalScopeParams,
  signal?: AbortSignal
): Promise<string> {
  const url = withQuery(
    `${workspaceRoot(workspaceId)}/recordings/${encodeURIComponent(recordingId)}`,
    terminalScopeQuery({ profile: scope.profile })
  );
  const response = await runtimeFetch(url, { method: "GET", signal });
  if (!response.ok) {
    throw new TerminalApiError(
      `Failed to load the recording: ${response.status}`,
      response.status,
      ""
    );
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
    afterSeq?: number;
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

/** The catalog SSE URL, workspace- and profile-scoped. */
export function terminalCatalogStreamPath(
  workspaceId: string,
  profile: string,
  allProfiles = false
): string {
  const query = terminalScopeQuery(allProfiles ? { all_profiles: true } : { profile });
  return `/api/workspaces/${encodeURIComponent(workspaceId)}/terminals/stream?${query.toString()}`;
}
