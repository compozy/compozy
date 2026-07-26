import type {
  HostAPIMethod,
  ExtensionToolCallRequest,
  JSONValue,
  ShutdownRequest,
  ToolID,
  ToolResult,
} from "./types.js";
import { CapabilityDeniedError, InvalidParamsError, ToolExecutionError } from "./errors.js";
import { cloneJSON } from "./json.js";

export function normalizeStringList(values: readonly string[] | undefined): string[] {
  return Array.from(new Set((values ?? []).map(value => value.trim()).filter(Boolean))).sort();
}

export function normalizeHostMethodList(
  values: readonly HostAPIMethod[] | undefined
): HostAPIMethod[] {
  return normalizeStringList(values) as HostAPIMethod[];
}

export function ensureSubset(label: string, requested: string[], granted: readonly string[]): void {
  const grantedSet = new Set(granted.map(value => value.trim()));
  const missing = requested.filter(value => !grantedSet.has(value));
  if (missing.length > 0) {
    throw new CapabilityDeniedError({
      field: label,
      required: missing,
      granted: [...grantedSet],
    });
  }
}

export function parseShutdownRequest(params: unknown): ShutdownRequest {
  if (typeof params !== "object" || params === null) {
    throw new InvalidParamsError("shutdown params must be an object");
  }
  const request = params as ShutdownRequest;
  if (!Number.isFinite(request.deadline_ms) || request.deadline_ms <= 0) {
    throw new InvalidParamsError("deadline_ms must be greater than zero");
  }
  return {
    reason: request.reason ?? "shutdown",
    deadline_ms: request.deadline_ms,
  };
}

export function parseToolCallRequest(params: unknown): ExtensionToolCallRequest {
  if (typeof params !== "object" || params === null) {
    throw new InvalidParamsError("tools/call params must be an object");
  }
  const request = params as ExtensionToolCallRequest;
  if (typeof request.tool_id !== "string" || request.tool_id.trim() === "") {
    throw new InvalidParamsError("tool_id is required");
  }
  if (typeof request.handler !== "string" || request.handler.trim() === "") {
    throw new InvalidParamsError("handler is required");
  }
  return {
    tool_id: request.tool_id.trim(),
    handler: request.handler.trim(),
    ...(request.session_id ? { session_id: request.session_id } : {}),
    input: (request.input ?? {}) as JSONValue,
  };
}

export function normalizeToolResult(value: ToolResult | JSONValue | string | void): ToolResult {
  if (value === undefined) {
    return emptyToolResult();
  }
  if (isToolResult(value)) {
    return { ...emptyToolResult(), ...value };
  }
  if (typeof value === "string") {
    return {
      ...emptyToolResult(),
      content: [{ type: "text", text: value }],
      preview: value,
    };
  }
  return {
    ...emptyToolResult(),
    structured: value,
  };
}

function emptyToolResult(): ToolResult {
  return {
    truncated: false,
    bytes: 0,
    duration_ms: 0,
  };
}

function isToolResult(value: ToolResult | JSONValue | string | void): value is ToolResult {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    return false;
  }
  return [
    "content",
    "structured",
    "preview",
    "artifacts",
    "metadata",
    "redactions",
    "truncated",
    "bytes",
    "duration_ms",
  ].some(key => Object.prototype.hasOwnProperty.call(value, key));
}

export function toolExecutionError(
  error: unknown,
  call: ExtensionToolCallRequest,
  sensitiveInputFields: string[]
): ToolExecutionError {
  const errorMessage = redactSensitiveText(
    error instanceof Error ? error.message : String(error),
    call.input,
    sensitiveInputFields
  );
  return new ToolExecutionError({
    tool_id: call.tool_id,
    handler: call.handler,
    error: errorMessage,
    ...(sensitiveInputFields.length > 0
      ? {
          input: redactSensitiveInput(call.input, sensitiveInputFields),
          sensitive_input_fields: sensitiveInputFields,
        }
      : {}),
  });
}

function redactSensitiveInput(value: JSONValue, sensitiveInputFields: string[]): JSONValue {
  const cloned = cloneJSON(value);
  for (const field of sensitiveInputFields) {
    redactPath(
      cloned,
      field
        .split(".")
        .map(part => part.trim())
        .filter(Boolean)
    );
  }
  return cloned;
}

function redactPath(value: JSONValue, path: string[]): void {
  if (path.length === 0 || typeof value !== "object" || value === null || Array.isArray(value)) {
    return;
  }
  const [head, ...rest] = path;
  if (head === undefined) {
    return;
  }
  if (!(head in value)) {
    return;
  }
  if (rest.length === 0) {
    value[head] = "[REDACTED]";
    return;
  }
  redactPath(value[head] as JSONValue, rest);
}

function redactSensitiveText(
  text: string,
  input: JSONValue,
  sensitiveInputFields: string[]
): string {
  let redacted = text;
  for (const field of sensitiveInputFields) {
    const value = readPath(
      input,
      field
        .split(".")
        .map(part => part.trim())
        .filter(Boolean)
    );
    if (typeof value === "string" && value !== "") {
      redacted = redacted.replaceAll(value, "[REDACTED]");
    }
  }
  return redacted;
}

function readPath(value: JSONValue, path: string[]): JSONValue | undefined {
  if (path.length === 0) {
    return value;
  }
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    return undefined;
  }
  const [head, ...rest] = path;
  if (head === undefined) {
    return undefined;
  }
  if (!(head in value)) {
    return undefined;
  }
  return readPath(value[head] as JSONValue, rest);
}

export function normalizeSchema(value: JSONValue, field: string): JSONValue {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    throw new Error(`${field} must be a JSON object`);
  }
  return cloneJSON(value);
}

export function canonicalExtensionToolID(extensionName: string, handler: string): ToolID {
  return `ext__${canonicalIDSegment(extensionName)}__${canonicalIDSegment(handler)}`;
}

function canonicalIDSegment(raw: string): string {
  const normalized = raw.trim().toLowerCase();
  let output = "";
  let lastUnderscore = false;
  for (const char of normalized) {
    if ((char >= "a" && char <= "z") || (char >= "0" && char <= "9")) {
      output += char;
      lastUnderscore = false;
      continue;
    }
    if (output.length > 0 && !lastUnderscore) {
      output += "_";
      lastUnderscore = true;
    }
  }
  const segment = output.replaceAll(/^_+|_+$/g, "");
  if (!/^[a-z][a-z0-9_]*$/.test(segment) || segment.includes("__")) {
    throw new Error(`invalid tool id segment: ${raw}`);
  }
  return segment;
}
