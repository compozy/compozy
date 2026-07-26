import type { JSONRPCErrorObject, JSONValue } from "./types.js";

export interface CapabilityDeniedData {
  method?: string;
  required?: string[];
  granted?: string[];
  field?: string;
}

export interface RateLimitedData {
  scope?: string;
  retry_after_ms?: number;
  limit?: number;
  burst?: number;
}

export interface AllowedMethodsData {
  allowed_methods?: string[];
}

export interface ShutdownProgressData {
  deadline_ms?: number;
}

export interface ToolExecutionData {
  tool_id?: string;
  handler?: string;
  error?: string;
  input?: JSONValue;
  sensitive_input_fields?: string[];
}

export class RPCError<Data = unknown> extends Error {
  readonly code: number;
  readonly data: Data | undefined;

  public constructor(code: number, message: string, data?: Data) {
    super(message);
    this.name = new.target.name;
    this.code = code;
    this.data = data;
  }

  public toJSONRPC(): JSONRPCErrorObject<Data> {
    const base = {
      code: this.code,
      message: this.message,
    };
    if (this.data === undefined) {
      return base;
    }
    return {
      ...base,
      data: this.data,
    };
  }
}

export class ParseError extends RPCError<Record<string, JSONValue>> {
  public constructor(error: string) {
    super(-32700, "Parse error", { error });
  }
}

export class InvalidRequestError extends RPCError<Record<string, JSONValue>> {
  public constructor(reason: string) {
    super(-32600, "Invalid request", { reason });
  }
}

export class MethodNotFoundError extends RPCError<Record<string, string>> {
  public constructor(method: string) {
    super(-32601, "Method not found", { method });
  }
}

export class InvalidParamsError extends RPCError<Record<string, JSONValue>> {
  public constructor(error: string, data?: Record<string, JSONValue>) {
    super(-32602, "Invalid params", { error, ...data });
  }
}

export class InternalError extends RPCError<Record<string, JSONValue>> {
  public constructor(error: string) {
    super(-32603, "Internal error", { error });
  }
}

export class CapabilityDeniedError extends RPCError<CapabilityDeniedData> {
  public constructor(data: CapabilityDeniedData) {
    super(-32001, "Capability denied", data);
  }
}

export class RateLimitedError extends RPCError<RateLimitedData> {
  public constructor(data: RateLimitedData) {
    super(-32002, "Rate limited", data);
  }
}

export class NotInitializedError extends RPCError<AllowedMethodsData> {
  public constructor(data: AllowedMethodsData = { allowed_methods: ["initialize"] }) {
    super(-32003, "Not initialized", data);
  }
}

export class ShutdownInProgressError extends RPCError<ShutdownProgressData> {
  public constructor(data: ShutdownProgressData = {}) {
    super(-32004, "Shutdown in progress", data);
  }
}

export class ToolExecutionError extends RPCError<ToolExecutionData> {
  public constructor(data: ToolExecutionData = {}) {
    super(-32010, "Tool execution failed", data);
  }
}

export function isRPCError(error: unknown): error is RPCError {
  return error instanceof RPCError;
}

export function errorFromObject(error: JSONRPCErrorObject): RPCError {
  switch (error.code) {
    case -32601:
      return new MethodNotFoundError(
        String((error.data as { method?: unknown } | undefined)?.method ?? "")
      );
    case -32602:
      return new InvalidParamsError(
        String((error.data as { error?: unknown } | undefined)?.error ?? error.message),
        (error.data as Record<string, JSONValue> | undefined) ?? {}
      );
    case -32001:
      return new CapabilityDeniedError((error.data as CapabilityDeniedData | undefined) ?? {});
    case -32002:
      return new RateLimitedError((error.data as RateLimitedData | undefined) ?? {});
    case -32003:
      return new NotInitializedError((error.data as AllowedMethodsData | undefined) ?? {});
    case -32004:
      return new ShutdownInProgressError((error.data as ShutdownProgressData | undefined) ?? {});
    case -32010:
      return new ToolExecutionError((error.data as ToolExecutionData | undefined) ?? {});
    default:
      return new RPCError(error.code, error.message, error.data);
  }
}

export function ensureRPCError(error: unknown): RPCError {
  if (error instanceof RPCError) {
    return error;
  }
  if (error instanceof Error) {
    return new InternalError(error.message);
  }
  return new InternalError(String(error));
}
