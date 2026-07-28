import { InvalidParamsError } from "./errors.js";
import { SUPPORTED_PROTOCOL_VERSIONS } from "./extension-contract.js";
import type { InitializeRequest } from "./types.js";

export function parseInitializeRequest(params: unknown): InitializeRequest {
  if (typeof params !== "object" || params === null) {
    throw new InvalidParamsError("initialize params must be an object");
  }

  const request = params as InitializeRequest;
  if (!request.protocol_version) {
    throw new InvalidParamsError("protocol_version is required");
  }
  if (typeof request.session_nonce !== "string" || request.session_nonce.trim() === "") {
    throw new InvalidParamsError("session_nonce is required");
  }
  if (
    !Array.isArray(request.supported_protocol_versions) ||
    request.supported_protocol_versions.length === 0
  ) {
    throw new InvalidParamsError("supported_protocol_versions is required");
  }
  if (!request.extension?.name || !request.extension?.version) {
    throw new InvalidParamsError("extension identity is required");
  }
  if (!request.capabilities || typeof request.capabilities !== "object") {
    throw new InvalidParamsError("capabilities are required");
  }
  if (!Array.isArray(request.capabilities.provides)) {
    throw new InvalidParamsError("capabilities.provides must be an array");
  }
  if (!Array.isArray(request.capabilities.granted_actions)) {
    throw new InvalidParamsError("capabilities.granted_actions must be an array");
  }
  if (!Array.isArray(request.capabilities.granted_security)) {
    throw new InvalidParamsError("capabilities.granted_security must be an array");
  }
  if (!Array.isArray(request.capabilities.granted_resource_kinds)) {
    throw new InvalidParamsError("capabilities.granted_resource_kinds must be an array");
  }
  if (!Array.isArray(request.capabilities.granted_resource_scopes)) {
    throw new InvalidParamsError("capabilities.granted_resource_scopes must be an array");
  }
  if (!request.runtime) {
    throw new InvalidParamsError("runtime is required");
  }
  for (const field of [
    "health_check_interval_ms",
    "health_check_timeout_ms",
    "shutdown_timeout_ms",
    "default_hook_timeout_ms",
  ] as const) {
    if (!Number.isFinite(request.runtime[field]) || request.runtime[field] <= 0) {
      throw new InvalidParamsError(`${field} must be greater than zero`);
    }
  }
  return request;
}

export function validateProtocolVersion(request: InitializeRequest): void {
  if (!SUPPORTED_PROTOCOL_VERSIONS.includes(request.protocol_version)) {
    throw new InvalidParamsError("unsupported protocol version", {
      reason: "unsupported_protocol_version",
      protocol_version: request.protocol_version,
      supported_protocol_versions: SUPPORTED_PROTOCOL_VERSIONS,
    });
  }
  if (!request.supported_protocol_versions.includes("1")) {
    throw new InvalidParamsError("supported protocol versions must include 1");
  }
}
