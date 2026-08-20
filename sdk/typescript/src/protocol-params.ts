import { InvalidParamsError } from "./errors.js";

export function isRequestRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

export function requestRecord(method: string, params: unknown): Record<string, unknown> {
  if (!isRequestRecord(params)) {
    throw new InvalidParamsError(`${method} params must be an object`);
  }
  return params;
}

export function requiredString(
  method: string,
  record: Record<string, unknown>,
  field: string
): string {
  const value = record[field];
  if (typeof value !== "string" || value.length === 0) {
    throw new InvalidParamsError(`${method} requires a non-empty ${field}`);
  }
  return value;
}
