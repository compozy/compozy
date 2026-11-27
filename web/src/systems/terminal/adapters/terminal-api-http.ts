import { clientID, computerApiBaseUrl, daemonFetch, type ComputerId } from "@/lib/api-client";
import type { ZodType } from "zod";

import {
  remoteTerminalErrorEnvelopeSchema,
  terminalErrorCodeSchema,
  terminalErrorEnvelopeSchema,
  type TerminalErrorCode,
  type TerminalErrorDetails,
} from "../lib/terminal-contract-schema";
import type { TerminalScopeParams, TerminalViewerIdentity } from "../types";

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

export function terminalWorkspaceRoot(workspaceId: string, computerId?: ComputerId): string {
  return `${computerApiBaseUrl(computerId)}/api/workspaces/${encodeURIComponent(workspaceId)}/terminals`;
}

export function terminalResourceURL(
  workspaceId: string,
  terminalId: string,
  suffix = "",
  computerId?: ComputerId
): string {
  return `${terminalWorkspaceRoot(workspaceId, computerId)}/${encodeURIComponent(terminalId)}${suffix}`;
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

export function withTerminalQuery(url: string, query: URLSearchParams): string {
  const serialized = query.toString();
  return serialized === "" ? url : `${url}?${serialized}`;
}

/** Serializes viewer proof using its registration partition, independently of the terminal destination. */
export function terminalViewerHeaders(viewer: TerminalViewerIdentity): Record<string, string> {
  return {
    "X-Compozy-Client-Token": viewer.attachmentToken,
    ...(viewer.workspaceId ? { "X-Compozy-Client-Workspace": viewer.workspaceId } : {}),
  };
}

export async function readTerminalBody(response: Response): Promise<unknown> {
  const text = await response.text();
  if (text.trim() === "") return undefined;
  try {
    return JSON.parse(text) as unknown;
  } catch {
    throw new TerminalProtocolError(response.status);
  }
}

export function terminalResponseError(response: Response, body: unknown): TerminalApiError {
  const parsed = terminalErrorEnvelopeSchema.safeParse(body);
  if (parsed.success) {
    return new TerminalApiError(
      parsed.data.error.message,
      response.status,
      parsed.data.error.code,
      parsed.data.error.details
    );
  }
  const remoteParsed = remoteTerminalErrorEnvelopeSchema.safeParse(body);
  if (remoteParsed.success) {
    return new TerminalApiError(
      remoteParsed.data.error,
      response.status,
      remoteParsed.data.code,
      remoteParsed.data.details
    );
  }
  throw new TerminalProtocolError(response.status);
}

/** Attributes terminal requests to this browser, maps domain failures, and validates successful payloads. */
export async function parseTerminalResponse<Result>(
  url: string,
  schema: ZodType<Result>,
  init?: RequestInit
): Promise<Result> {
  const response = await daemonFetch(url, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      "X-Compozy-Client-ID": clientID,
      ...init?.headers,
    },
  });
  const body = await readTerminalBody(response);
  if (!response.ok) {
    throw terminalResponseError(response, body);
  }
  const parsed = schema.safeParse(body);
  if (!parsed.success) throw new TerminalProtocolError(response.status);
  return parsed.data;
}
