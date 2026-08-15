/**
 * Client-side wait-resume payload check. The daemon remains the authority; this
 * only disables confirm while the typed JSON cannot satisfy the wait's `expect`.
 */

export interface LoopWaitPayloadCheck {
  ok: boolean;
  /** Field hint: the wait's declared expect, serialized. */
  hint: string;
  /** Names the first missing key or a JSON parse failure. */
  error?: string;
}

const JSON_SCHEMA_TYPES = new Set([
  "object",
  "array",
  "string",
  "number",
  "integer",
  "boolean",
  "null",
]);

function asRecord(value: unknown): Record<string, unknown> | null {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;
}

function expectHint(expect: unknown): string {
  if (expect === undefined || expect === null) return "";
  return JSON.stringify(expect);
}

function isJsonSchema(record: Record<string, unknown>): boolean {
  return (
    (typeof record.type === "string" && JSON_SCHEMA_TYPES.has(record.type)) ||
    asRecord(record.properties) !== null ||
    Array.isArray(record.required)
  );
}

/** Required keys from a JSON Schema `required` list, or from a sample object. */
export function loopWaitExpectRequiredKeys(expect: unknown): string[] {
  const record = asRecord(expect);
  if (!record) return [];
  if (Array.isArray(record.required)) {
    return record.required.filter((key): key is string => typeof key === "string" && key !== "");
  }
  if (isJsonSchema(record)) return [];
  return Object.keys(record);
}

export function checkLoopWaitPayload(payload: string, expect: unknown): LoopWaitPayloadCheck {
  const hint = expectHint(expect);
  let parsed: unknown;
  try {
    parsed = payload.trim() === "" ? {} : JSON.parse(payload);
  } catch {
    return { ok: false, hint, error: "Payload must be JSON." };
  }
  if (expect === undefined || expect === null) return { ok: true, hint };
  const schema = asRecord(expect);
  if (schema?.type === "object" && asRecord(parsed) === null) {
    return { ok: false, hint, error: "Payload must be an object." };
  }
  const required = loopWaitExpectRequiredKeys(expect);
  if (required.length === 0) return { ok: true, hint };
  const body = asRecord(parsed);
  if (!body) {
    return { ok: false, hint, error: `Missing key ${required[0]}.` };
  }
  const missing = required.find(key => !(key in body));
  if (missing) return { ok: false, hint, error: `Missing key ${missing}.` };
  return { ok: true, hint };
}
