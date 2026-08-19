import { apiClient, apiErrorMessage, apiRequestFailed } from "@/lib/api-client";

import type {
  CmdPaletteAttachedClient,
  CmdPaletteCatalogResponse,
  CmdPaletteInvokeResult,
  CmdPalettePersonalization,
  CmdPaletteRankSignals,
} from "../lib/cmd-palette-types";

export type CmdPaletteApiErrorKind = "daemon" | "malformed_response" | "transport";

export interface CmdPaletteApiErrorMetadata {
  /** Daemon error code, e.g. `no_attached_shell`, `multiple_clients`, `already_running`. */
  readonly code?: string;
  /** Verbatim availability reason — the same string the catalog carries (BR-8). */
  readonly reason?: string;
  /** Attached client ids the daemon offers when targeting is ambiguous. */
  readonly clients?: readonly string[];
  /** Per-argument validation messages from a 422. */
  readonly fields?: Readonly<Record<string, string>>;
}

export class CmdPaletteApiError extends Error {
  public readonly code: string | undefined;
  public readonly reason: string | undefined;
  public readonly clients: readonly string[] | undefined;
  public readonly fields: Readonly<Record<string, string>> | undefined;

  constructor(
    message: string,
    public readonly status: number,
    public readonly kind: CmdPaletteApiErrorKind,
    metadata: CmdPaletteApiErrorMetadata = {}
  ) {
    super(message);
    this.name = "CmdPaletteApiError";
    this.code = metadata.code;
    this.reason = metadata.reason;
    this.clients = metadata.clients;
    this.fields = metadata.fields;
  }
}

function errorString(error: unknown, field: string): string | undefined {
  if (error == null || typeof error !== "object") return undefined;
  const value = Reflect.get(error, field);
  if (typeof value !== "string") return undefined;
  const normalized = value.trim();
  return normalized === "" ? undefined : normalized;
}

function errorClients(error: unknown): readonly string[] | undefined {
  if (error == null || typeof error !== "object") return undefined;
  const value = Reflect.get(error, "clients");
  if (!Array.isArray(value)) return undefined;
  const clients = value.filter((entry): entry is string => typeof entry === "string");
  return clients.length === 0 ? undefined : clients;
}

function errorFields(error: unknown): Readonly<Record<string, string>> | undefined {
  if (error == null || typeof error !== "object") return undefined;
  const value = Reflect.get(error, "fields");
  if (value == null || typeof value !== "object" || Array.isArray(value)) return undefined;
  const fields: Record<string, string> = {};
  for (const [key, entry] of Object.entries(value)) {
    if (typeof entry === "string") fields[key] = entry;
  }
  return Object.keys(fields).length === 0 ? undefined : fields;
}

function daemonErrorMetadata(error: unknown): CmdPaletteApiErrorMetadata {
  const code = errorString(error, "error");
  const reason = errorString(error, "reason");
  const clients = errorClients(error);
  const fields = errorFields(error);
  return {
    ...(code ? { code } : {}),
    ...(reason ? { reason } : {}),
    ...(clients ? { clients } : {}),
    ...(fields ? { fields } : {}),
  };
}

function responseError(fallback: string, response: Response, error: unknown): CmdPaletteApiError {
  const metadata = daemonErrorMetadata(error);
  // The daemon's `message` carries the operator-facing sentence; `error` is the
  // stable code. Prefer the message, fall back to the reason, then the code.
  const daemonMessage =
    errorString(error, "message") ?? metadata.reason ?? apiErrorMessage(error) ?? undefined;
  return new CmdPaletteApiError(
    daemonMessage ?? `${fallback} (${response.status})`,
    response.status,
    daemonMessage === undefined ? "transport" : "daemon",
    metadata
  );
}

function responseData<T>(data: T | undefined, response: Response, fallback: string): T {
  if (data === undefined) {
    throw new CmdPaletteApiError(
      `${fallback}: empty response (${response.status})`,
      response.status,
      "malformed_response"
    );
  }
  return data;
}

function requireArray<T>(
  value: T[] | undefined,
  response: Response,
  fallback: string,
  field: string
): T[] {
  if (!Array.isArray(value)) {
    throw new CmdPaletteApiError(
      `${fallback}: missing ${field} (${response.status})`,
      response.status,
      "malformed_response"
    );
  }
  return value;
}

/**
 * The catalog for one workspace, resolved against one attached client.
 *
 * `client` is what makes client-context predicates resolvable: listing without
 * it reports `requires an attached shell` rather than auto-selecting an
 * attachment (Part II § API Endpoints).
 */
export async function listCmdPaletteCommands(
  workspaceId: string,
  clientId: string | null,
  signal?: AbortSignal
): Promise<CmdPaletteCatalogResponse> {
  const client = clientId?.trim() || undefined;
  const { data, error, response } = await apiClient.GET("/api/cmd-palette/commands", {
    params: { query: { workspace: workspaceId, ...(client ? { client } : {}) } },
    signal,
  });
  const fallback = "Failed to load the command catalog";
  if (apiRequestFailed(response, error)) throw responseError(fallback, response, error);
  const envelope = responseData(data, response, fallback);
  requireArray(envelope.commands, response, fallback, "commands");
  requireArray(envelope.sources, response, fallback, "sources");
  if (typeof envelope.catalog_revision !== "string" || envelope.catalog_revision.trim() === "") {
    throw new CmdPaletteApiError(
      `${fallback}: missing catalog_revision (${response.status})`,
      response.status,
      "malformed_response"
    );
  }
  return envelope;
}

export async function listCmdPaletteClients(
  workspaceId: string,
  signal?: AbortSignal
): Promise<CmdPaletteAttachedClient[]> {
  const { data, error, response } = await apiClient.GET("/api/cmd-palette/clients", {
    params: { query: { workspace: workspaceId } },
    signal,
  });
  const fallback = "Failed to list attached clients";
  if (apiRequestFailed(response, error)) throw responseError(fallback, response, error);
  const envelope = responseData(data, response, fallback);
  return requireArray(envelope, response, fallback, "clients");
}

export interface CmdPaletteInvokeInput {
  readonly workspaceId: string;
  readonly commandId: string;
  readonly args?: Readonly<Record<string, unknown>>;
  readonly clientId?: string | null;
}

/**
 * Runs a daemon-executed command. Client-executed operations never come through
 * here — the seam runs them locally against the coordinators (ADR-006).
 */
export async function invokeCmdPaletteCommand(
  input: CmdPaletteInvokeInput,
  signal?: AbortSignal
): Promise<CmdPaletteInvokeResult> {
  const client = input.clientId?.trim() || undefined;
  const { data, error, response } = await apiClient.POST("/api/cmd-palette/commands/{id}/invoke", {
    params: { path: { id: input.commandId } },
    body: {
      workspace: input.workspaceId,
      args: { ...input.args },
      ...(client ? { client } : {}),
    },
    signal,
  });
  const fallback = `Failed to run ${input.commandId}`;
  if (apiRequestFailed(response, error)) throw responseError(fallback, response, error);
  return responseData(data, response, fallback);
}

export async function getCmdPaletteRankSignals(
  workspaceId: string,
  signal?: AbortSignal
): Promise<CmdPaletteRankSignals> {
  const { data, error, response } = await apiClient.GET("/api/cmd-palette/rank-signals", {
    params: { query: { workspace: workspaceId } },
    signal,
  });
  const fallback = "Failed to load command palette rank signals";
  if (apiRequestFailed(response, error)) throw responseError(fallback, response, error);
  return responseData(data, response, fallback);
}

export async function recordCmdPaletteUsage(
  workspaceId: string,
  commandId: string,
  query: string,
  signal?: AbortSignal
): Promise<void> {
  const { error, response } = await apiClient.POST("/api/cmd-palette/usage", {
    body: { workspace: workspaceId, command_id: commandId, query },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw responseError("Failed to record command palette usage", response, error);
  }
}

export async function setCmdPalettePin(
  workspaceId: string,
  commandId: string,
  pinned: boolean,
  signal?: AbortSignal
): Promise<boolean> {
  const request = pinned ? apiClient.PUT : apiClient.DELETE;
  const { data, error, response } = await request("/api/cmd-palette/pins/{id}", {
    params: { path: { id: commandId }, query: { workspace: workspaceId } },
    signal,
  });
  const fallback = pinned ? "Failed to pin command" : "Failed to unpin command";
  if (apiRequestFailed(response, error)) throw responseError(fallback, response, error);
  return responseData(data, response, fallback).pinned;
}

export async function getCmdPalettePersonalization(
  workspaceId: string,
  signal?: AbortSignal
): Promise<CmdPalettePersonalization> {
  const { data, error, response } = await apiClient.GET("/api/cmd-palette/personalization", {
    params: { query: { workspace: workspaceId } },
    signal,
  });
  const fallback = "Failed to load command palette personalization";
  if (apiRequestFailed(response, error)) throw responseError(fallback, response, error);
  return responseData(data, response, fallback);
}

export async function resetCmdPalettePersonalization(
  workspaceId: string,
  signal?: AbortSignal
): Promise<void> {
  const { error, response } = await apiClient.DELETE("/api/cmd-palette/personalization", {
    params: { query: { workspace: workspaceId } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw responseError("Failed to reset command palette personalization", response, error);
  }
}
