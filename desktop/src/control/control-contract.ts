export const CONTROL_SCHEMA_VERSION = 1;

export type ControlMethod =
  | "navigate"
  | "retry"
  | "diagnose"
  | "export_diagnostics"
  | "copy_diagnostics";

export interface ControlRequest {
  readonly schema_version: 1;
  readonly id: number;
  readonly token: string;
  readonly method: string;
  readonly params?: unknown;
}

export interface ControlErrorPayload {
  readonly code: string;
  readonly message: string;
}

export type ControlResponse =
  | { readonly schema_version: 1; readonly id: number; readonly result: unknown }
  | { readonly schema_version: 1; readonly id: number; readonly error: ControlErrorPayload };

export type ControlHandler = (method: ControlMethod, params: unknown) => Promise<unknown>;

export class ControlMethodError extends Error {
  readonly code: string;

  constructor(code: string, message: string) {
    super(message);
    this.name = "ControlMethodError";
    this.code = code;
  }
}

const METHODS = new Set<ControlMethod>([
  "navigate",
  "retry",
  "diagnose",
  "export_diagnostics",
  "copy_diagnostics",
]);

export function parseControlMethod(value: string): ControlMethod | null {
  return METHODS.has(value as ControlMethod) ? (value as ControlMethod) : null;
}
