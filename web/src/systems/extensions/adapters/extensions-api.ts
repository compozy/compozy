import { apiClient, apiErrorMessage, apiRequestFailed } from "@/lib/api-client";

import type {
  BundleActivation,
  BundleActivationUpdateRequest,
  ExtensionEntry,
  ExtensionProvenance,
  ExtensionUpdateRequest,
} from "../types";

export type ExtensionsApiErrorKind = "daemon" | "malformed_response" | "transport";

export class ExtensionsApiError extends Error {
  constructor(
    message: string,
    public readonly status: number,
    public readonly kind: ExtensionsApiErrorKind
  ) {
    super(message);
    this.name = "ExtensionsApiError";
  }
}

function responseError(fallback: string, response: Response, error: unknown): ExtensionsApiError {
  const daemonMessage = apiErrorMessage(error);
  return new ExtensionsApiError(
    daemonMessage ?? (response.status ? `${fallback} (${response.status})` : fallback),
    response.status,
    daemonMessage ? "daemon" : "transport"
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

export async function listExtensions(signal?: AbortSignal): Promise<ExtensionEntry[]> {
  const { data, error, response } = await apiClient.GET("/api/extensions", { signal });
  if (apiRequestFailed(response, error))
    throw responseError("Failed to list extensions", response, error);
  const fallback = "Failed to list extensions";
  const envelope = responseData(data, response, fallback);
  return requiredArray(envelope.extensions, response, fallback, "extensions");
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

async function setExtensionEnabled(
  name: string,
  enabled: boolean,
  signal?: AbortSignal
): Promise<ExtensionEntry> {
  const path = enabled ? "/api/extensions/{name}/enable" : "/api/extensions/{name}/disable";
  const { data, error, response } = await apiClient.POST(path, {
    params: { path: { name } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw responseError(`Failed to ${enabled ? "enable" : "disable"} ${name}`, response, error);
  }
  const fallback = `Failed to update ${name}`;
  const envelope = responseData(data, response, fallback);
  return requiredObject(envelope.extension, response, fallback, "extension");
}

export function enableExtension(name: string, signal?: AbortSignal): Promise<ExtensionEntry> {
  return setExtensionEnabled(name, true, signal);
}

export function disableExtension(name: string, signal?: AbortSignal): Promise<ExtensionEntry> {
  return setExtensionEnabled(name, false, signal);
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

export async function removeExtension(name: string, signal?: AbortSignal): Promise<void> {
  const { error, response } = await apiClient.DELETE("/api/extensions/{name}", {
    params: { path: { name } },
    signal,
  });
  if (apiRequestFailed(response, error))
    throw responseError(`Failed to remove ${name}`, response, error);
}

export async function listBundleActivations(signal?: AbortSignal): Promise<BundleActivation[]> {
  const { data, error, response } = await apiClient.GET("/api/bundles/activations", { signal });
  if (apiRequestFailed(response, error)) {
    throw responseError("Failed to list bundle activations", response, error);
  }
  const fallback = "Failed to list bundle activations";
  const envelope = responseData(data, response, fallback);
  return requiredArray(envelope.activations, response, fallback, "activations");
}

export async function getBundleActivation(
  id: string,
  signal?: AbortSignal
): Promise<BundleActivation> {
  const { data, error, response } = await apiClient.GET("/api/bundles/activations/{id}", {
    params: { path: { id } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw responseError(`Failed to load bundle activation ${id}`, response, error);
  }
  const fallback = `Failed to load bundle activation ${id}`;
  const envelope = responseData(data, response, fallback);
  return requiredObject(envelope.activation, response, fallback, "activation");
}

export async function updateBundleActivation(
  id: string,
  body: BundleActivationUpdateRequest,
  signal?: AbortSignal
): Promise<BundleActivation> {
  const { data, error, response } = await apiClient.PATCH("/api/bundles/activations/{id}", {
    params: { path: { id } },
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw responseError(`Failed to update bundle activation ${id}`, response, error);
  }
  const fallback = `Failed to update bundle activation ${id}`;
  const envelope = responseData(data, response, fallback);
  return requiredObject(envelope.activation, response, fallback, "activation");
}

export async function deactivateBundle(id: string, signal?: AbortSignal): Promise<void> {
  const { error, response } = await apiClient.DELETE("/api/bundles/activations/{id}", {
    params: { path: { id } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw responseError(`Failed to deactivate bundle activation ${id}`, response, error);
  }
}
