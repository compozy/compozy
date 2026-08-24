import { apiClient, apiErrorMessage, apiRequestFailed } from "@/lib/api-client";

import type {
  ExtensionEnablement,
  ExtensionEntry,
  ExtensionInstallRequest,
  ExtensionInstallPreview,
  ExtensionInstanceScope,
  ExtensionKitInventory,
  ExtensionLogsSnapshot,
  ExtensionProvenance,
  ExtensionUpdateRequest,
} from "../types";

/** Profile-aware instance selector used by inventory reads and lifecycle mutations. */
function instanceQuery(
  scope: ExtensionInstanceScope | undefined
): { profile?: string; workspace?: string } | undefined {
  const workspace = scope?.workspaceId?.trim() ?? "";
  const profile = scope?.profileName?.trim() ?? "";
  if (workspace === "" && profile === "") return undefined;
  return {
    ...(profile ? { profile } : {}),
    ...(workspace ? { workspace } : {}),
  };
}

/** The logs route is workspace-scoped; profile is not part of its generated contract. */
function workspaceQuery(
  scope: Pick<ExtensionInstanceScope, "workspaceId">
): { workspace?: string } | undefined {
  const workspace = scope.workspaceId?.trim() ?? "";
  return workspace === "" ? undefined : { workspace };
}

export type ExtensionsApiErrorKind = "daemon" | "malformed_response" | "transport";

export interface ExtensionsApiErrorMetadata {
  readonly code?: string;
  readonly currentDigest?: string;
}

export class ExtensionsApiError extends Error {
  /** Daemon error code, e.g. `extension_network_confirmation_required`. */
  public readonly code: string | undefined;

  /** Digest the daemon expects consent for; the remediation for a missing or stale confirm. */
  public readonly currentDigest: string | undefined;

  constructor(
    message: string,
    public readonly status: number,
    public readonly kind: ExtensionsApiErrorKind,
    metadata: ExtensionsApiErrorMetadata = {}
  ) {
    super(message);
    this.name = "ExtensionsApiError";
    this.code = metadata.code;
    this.currentDigest = metadata.currentDigest;
  }
}

type DaemonErrorField = "code" | "current_digest";

function errorField(error: unknown, field: DaemonErrorField): string | undefined {
  if (error == null || typeof error !== "object") return undefined;
  const value = Reflect.get(error, field);
  if (typeof value !== "string") return undefined;
  const normalized = value.trim();
  return normalized === "" ? undefined : normalized;
}

function daemonErrorMetadata(error: unknown): ExtensionsApiErrorMetadata {
  const code = errorField(error, "code");
  const currentDigest = errorField(error, "current_digest");
  return {
    ...(code ? { code } : {}),
    ...(currentDigest ? { currentDigest } : {}),
  };
}

function responseError(fallback: string, response: Response, error: unknown): ExtensionsApiError {
  const daemonMessage = apiErrorMessage(error);
  return new ExtensionsApiError(
    daemonMessage ?? (response.status ? `${fallback} (${response.status})` : fallback),
    response.status,
    daemonMessage ? "daemon" : "transport",
    daemonErrorMetadata(error)
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

function requiredString(
  value: string | undefined,
  response: Response,
  fallback: string,
  field: string
): string {
  if (typeof value !== "string" || value.trim() === "") {
    throw malformedField(fallback, field, response);
  }
  return value;
}

function requiredBoolean(
  value: boolean | undefined,
  response: Response,
  fallback: string,
  field: string
): boolean {
  if (typeof value !== "boolean") throw malformedField(fallback, field, response);
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
  options: ExtensionInstanceScope & { after?: number; streamEpoch?: string } = {},
  signal?: AbortSignal
): Promise<ExtensionLogsSnapshot> {
  const after =
    options.after !== undefined && options.after > 0 ? String(options.after) : undefined;
  const streamEpoch = options.streamEpoch?.trim() || undefined;
  const { data, error, response } = await apiClient.GET("/api/extensions/{name}/logs", {
    params: {
      path: { name },
      query: { ...workspaceQuery(options), after, stream_epoch: streamEpoch },
    },
    signal,
  });
  const fallback = `Failed to load logs for ${name}`;
  if (apiRequestFailed(response, error)) throw responseError(fallback, response, error);
  const envelope = responseData(data, response, fallback);
  return {
    logs: requiredArray(envelope.logs, response, fallback, "logs"),
    stream_epoch: requiredString(envelope.stream_epoch, response, fallback, "stream_epoch"),
  };
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

export async function setExtensionEnablement(
  name: string,
  profile: string,
  enabled: boolean,
  signal?: AbortSignal
): Promise<ExtensionEnablement> {
  const { data, error, response } = await apiClient.PUT("/api/extensions/{name}/enablement", {
    params: { path: { name } },
    body: { enabled, profile: profile.trim() },
    signal,
  });
  const fallback = `Failed to change ${name} in profile ${profile}`;
  if (apiRequestFailed(response, error)) throw responseError(fallback, response, error);
  const enablement = responseData(data, response, fallback);
  requiredString(enablement.profile, response, fallback, "profile");
  requiredBoolean(enablement.enabled, response, fallback, "enabled");
  return enablement;
}

export async function previewExtensionInstall(
  body: ExtensionInstallRequest,
  signal?: AbortSignal
): Promise<ExtensionInstallPreview> {
  const { data, error, response } = await apiClient.POST("/api/extensions/preview-install", {
    body,
    signal,
  });
  const fallback = "Failed to preview extension install";
  if (apiRequestFailed(response, error)) throw responseError(fallback, response, error);
  const preview = responseData(data, response, fallback);
  requiredString(preview.name, response, fallback, "name");
  requiredArray(preview.declared_profiles, response, fallback, "declared_profiles");
  requiredArray(preview.placements, response, fallback, "placements");
  return preview;
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
 * Shipped-vs-live kit resources for one extension. The route carries no instance selector: the
 * daemon answers for the global published instance.
 */
export async function getExtensionInventory(
  name: string,
  signal?: AbortSignal
): Promise<ExtensionKitInventory> {
  const { data, error, response } = await apiClient.GET("/api/extensions/{name}/inventory", {
    params: { path: { name } },
    signal,
  });
  const fallback = `Failed to load kit inventory for ${name}`;
  if (apiRequestFailed(response, error)) throw responseError(fallback, response, error);
  const envelope: ExtensionKitInventory = responseData(data, response, fallback);
  requiredArray(envelope.items, response, fallback, "items");
  const extension = requiredString(envelope.extension, response, fallback, "extension");
  requiredBoolean(envelope.enabled, response, fallback, "enabled");
  if (extension !== name) {
    throw new ExtensionsApiError(
      `${fallback}: extension identity mismatch (${response.status})`,
      response.status,
      "malformed_response"
    );
  }
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
