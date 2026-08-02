import { apiClient, apiErrorMessage, apiRequestFailed } from "@/lib/api-client";

import type {
  ExtensionEnableResult,
  ExtensionEntry,
  ExtensionInstanceScope,
  ExtensionInventory,
  ExtensionLogEntry,
  ExtensionProvenance,
  ExtensionUpdateRequest,
} from "../types";

/**
 * The daemon reads `?workspace=` as the instance selector: present resolves the caller's workspace
 * instance (dev overlay when linked), absent resolves the global published row.
 */
function instanceQuery(
  scope: ExtensionInstanceScope | undefined
): { workspace: string } | undefined {
  const workspace = scope?.workspaceId?.trim() ?? "";
  return workspace === "" ? undefined : { workspace };
}

export type ExtensionsApiErrorKind = "daemon" | "malformed_response" | "transport";

export class ExtensionsApiError extends Error {
  constructor(
    message: string,
    public readonly status: number,
    public readonly kind: ExtensionsApiErrorKind,
    /** Daemon error code, e.g. `extension_network_confirmation_required`. */
    public readonly code?: string,
    /** Digest the daemon expects consent for; the remediation for a missing or stale confirm. */
    public readonly currentDigest?: string
  ) {
    super(message);
    this.name = "ExtensionsApiError";
  }
}

function errorField(error: unknown, field: string): string | undefined {
  if (error == null || typeof error !== "object") return undefined;
  const value = Reflect.get(error, field);
  if (typeof value !== "string") return undefined;
  const normalized = value.trim();
  return normalized === "" ? undefined : normalized;
}

function responseError(fallback: string, response: Response, error: unknown): ExtensionsApiError {
  const daemonMessage = apiErrorMessage(error);
  return new ExtensionsApiError(
    daemonMessage ?? (response.status ? `${fallback} (${response.status})` : fallback),
    response.status,
    daemonMessage ? "daemon" : "transport",
    errorField(error, "code"),
    errorField(error, "current_digest")
  );
}

function responseData<T>(data: T | undefined, response: Response, fallback: string): T {
  if (data === undefined) {
    throw new ExtensionsApiError(
      `${fallback}: empty response (${response.status})`,
      response.status,
      "malformed_response"
    );
  }
  return data;
}

function malformedField(fallback: string, field: string, response: Response): ExtensionsApiError {
  return new ExtensionsApiError(
    `${fallback}: missing ${field} (${response.status})`,
    response.status,
    "malformed_response"
  );
}

function requiredArray<T>(
  value: T[] | undefined,
  response: Response,
  fallback: string,
  field: string
): T[] {
  if (!Array.isArray(value)) throw malformedField(fallback, field, response);
  return value;
}

function requiredObject<T extends object>(
  value: T | undefined,
  response: Response,
  fallback: string,
  field: string
): T {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    throw malformedField(fallback, field, response);
  }
  return value;
}

export async function listExtensions(
  scope?: ExtensionInstanceScope,
  signal?: AbortSignal
): Promise<ExtensionEntry[]> {
  const { data, error, response } = await apiClient.GET("/api/extensions", {
    params: { query: instanceQuery(scope) },
    signal,
  });
  if (apiRequestFailed(response, error))
    throw responseError("Failed to list extensions", response, error);
  const fallback = "Failed to list extensions";
  const envelope = responseData(data, response, fallback);
  return requiredArray(envelope.extensions, response, fallback, "extensions");
}

export async function listExtensionLogs(
  name: string,
  options: ExtensionInstanceScope & { after?: number } = {},
  signal?: AbortSignal
): Promise<ExtensionLogEntry[]> {
  const after =
    options.after !== undefined && options.after > 0 ? String(options.after) : undefined;
  const { data, error, response } = await apiClient.GET("/api/extensions/{name}/logs", {
    params: { path: { name }, query: { ...instanceQuery(options), after } },
    signal,
  });
  const fallback = `Failed to load logs for ${name}`;
  if (apiRequestFailed(response, error)) throw responseError(fallback, response, error);
  const envelope = responseData(data, response, fallback);
  return requiredArray(envelope.logs, response, fallback, "logs");
}

export async function getExtensionProvenance(
  name: string,
  signal?: AbortSignal
): Promise<ExtensionProvenance> {
  const { data, error, response } = await apiClient.GET("/api/extensions/{name}/provenance", {
    params: { path: { name } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw responseError(`Failed to load provenance for ${name}`, response, error);
  }
  const fallback = `Failed to load provenance for ${name}`;
  const envelope = responseData(data, response, fallback);
  return requiredObject(envelope.provenance, response, fallback, "provenance");
}

/**
 * Enable is the only kit switch: its result enumerates the automation definitions that became
 * runnable in the committed operation, so the caller receives the whole result rather than the
 * extension row alone.
 */
export async function enableExtension(
  name: string,
  options: { confirmNetworkDigest?: string } = {},
  signal?: AbortSignal
): Promise<ExtensionEnableResult> {
  const digest = options.confirmNetworkDigest?.trim();
  const { data, error, response } = await apiClient.POST("/api/extensions/{name}/enable", {
    params: { path: { name } },
    body: digest ? { confirm_network_digest: digest } : {},
    signal,
  });
  const fallback = `Failed to enable ${name}`;
  if (apiRequestFailed(response, error)) throw responseError(fallback, response, error);
  const envelope = responseData(data, response, fallback);
  requiredObject(envelope.extension, response, fallback, "extension");
  requiredArray(envelope.automation_started, response, fallback, "automation_started");
  return envelope;
}

export async function disableExtension(
  name: string,
  signal?: AbortSignal
): Promise<ExtensionEntry> {
  const { data, error, response } = await apiClient.POST("/api/extensions/{name}/disable", {
    params: { path: { name } },
    signal,
  });
  const fallback = `Failed to disable ${name}`;
  if (apiRequestFailed(response, error)) throw responseError(fallback, response, error);
  const envelope = responseData(data, response, fallback);
  return requiredObject(envelope.extension, response, fallback, "extension");
}

export async function updateExtension(
  name: string,
  body: ExtensionUpdateRequest,
  signal?: AbortSignal
): Promise<void> {
  const { error, response } = await apiClient.PUT("/api/extensions/{name}", {
    params: { path: { name } },
    body,
    signal,
  });
  if (apiRequestFailed(response, error))
    throw responseError(`Failed to update ${name}`, response, error);
}

/**
 * Shipped-vs-live kit resources for one extension. The route carries no workspace selector: the
 * daemon answers for the global published instance.
 */
export async function getExtensionInventory(
  name: string,
  signal?: AbortSignal
): Promise<ExtensionInventory> {
  const { data, error, response } = await apiClient.GET("/api/extensions/{name}/inventory", {
    params: { path: { name } },
    signal,
  });
  const fallback = `Failed to load kit inventory for ${name}`;
  if (apiRequestFailed(response, error)) throw responseError(fallback, response, error);
  const envelope: ExtensionInventory = responseData(data, response, fallback);
  requiredArray(envelope.items, response, fallback, "items");
  return envelope;
}

/** A workspace selects a dev unlink when present; an absent workspace removes the global row. */
export async function removeExtension(
  name: string,
  scope?: ExtensionInstanceScope,
  signal?: AbortSignal
): Promise<void> {
  const { error, response } = await apiClient.DELETE("/api/extensions/{name}", {
    params: { path: { name }, query: instanceQuery(scope) },
    signal,
  });
  if (apiRequestFailed(response, error))
    throw responseError(`Failed to remove ${name}`, response, error);
}
