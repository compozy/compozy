export const CONTROL_SCHEMA_VERSION = 1;

export const CONTROL_METHOD_VALUES = [
  "navigate",
  "retry",
  "diagnose",
  "export_diagnostics",
  "copy_diagnostics",
] as const;
export type ControlMethod = (typeof CONTROL_METHOD_VALUES)[number];

interface ControlEnvelope {
  readonly schema_version: typeof CONTROL_SCHEMA_VERSION;
  readonly id: number;
}

export interface ControlRequest extends ControlEnvelope {
  readonly token: string;
  readonly method: string;
  readonly params?: unknown;
}

export interface ControlErrorPayload {
  readonly code: string;
  readonly message: string;
}

export type ControlResponse =
  | (ControlEnvelope & { readonly result: unknown })
  | (ControlEnvelope & { readonly error: ControlErrorPayload });

export type ControlHandler = (method: ControlMethod, params: unknown) => Promise<unknown>;

export class ControlMethodError extends Error {
  readonly code: string;

  constructor(code: string, message: string) {
    super(message);
    this.name = "ControlMethodError";
    this.code = code;
  }
}

const METHODS = new Set<string>(CONTROL_METHOD_VALUES);

export function parseControlMethod(value: string): ControlMethod | null {
  return METHODS.has(value as ControlMethod) ? (value as ControlMethod) : null;
}
