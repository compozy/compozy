export type LoopRequestFieldType = "string" | "number" | "integer" | "boolean" | "json";

export interface LoopRequestField {
  name: string;
  type: LoopRequestFieldType;
  required: boolean;
  description: string;

  options: string[];
}

export interface LoopRequestPayloadCheck {
  ok: boolean;

  errors: Record<string, string>;

  payload?: Record<string, unknown>;
}

const FIELD_TYPES = new Set<LoopRequestFieldType>([
  "string",
  "number",
  "integer",
  "boolean",
  "json",
]);

function asRecord(value: unknown): Record<string, unknown> | null {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;
}

function stringList(value: unknown): string[] {
  if (!Array.isArray(value)) return [];
  return value.filter((entry): entry is string => typeof entry === "string");
}

function fieldType(schema: Record<string, unknown>): LoopRequestFieldType {
  const declared = typeof schema.type === "string" ? schema.type : "";
  return FIELD_TYPES.has(declared as LoopRequestFieldType)
    ? (declared as LoopRequestFieldType)
    : "json";
}

export function loopRequestFields(schema: unknown): LoopRequestField[] {
  const record = asRecord(schema);
  if (!record) return [];
  const properties = asRecord(record.properties);
  if (!properties) return [];
  const required = new Set(stringList(record.required));
  return Object.entries(properties).map(([name, raw]) => {
    const property = asRecord(raw) ?? {};
    return {
      name,
      type: fieldType(property),
      required: required.has(name),
      description: typeof property.description === "string" ? property.description : "",
      options: stringList(property.enum),
    };
  });
}

export function isLoopRequestFieldSchema(schema: unknown): boolean {
  return loopRequestFields(schema).length > 0;
}

function parseFieldValue(
  field: LoopRequestField,
  raw: string
): { value?: unknown; error?: string } {
  const trimmed = raw.trim();
  if (trimmed === "") {
    return field.required ? { error: `${field.name} is required.` } : {};
  }
  switch (field.type) {
    case "boolean":
      if (trimmed === "true") return { value: true };
      if (trimmed === "false") return { value: false };
      return { error: `${field.name} must be true or false.` };
    case "number":
    case "integer": {
      const parsed = Number(trimmed);
      if (!Number.isFinite(parsed)) return { error: `${field.name} must be a number.` };
      if (field.type === "integer" && !Number.isInteger(parsed)) {
        return { error: `${field.name} must be a whole number.` };
      }
      return { value: parsed };
    }
    case "string":
      if (field.options.length > 0 && !field.options.includes(trimmed)) {
        return { error: `${field.name} must be one of ${field.options.join(", ")}.` };
      }
      return { value: trimmed };
    default:
      try {
        return { value: JSON.parse(trimmed) as unknown };
      } catch {
        return { error: `${field.name} must be JSON.` };
      }
  }
}

export function checkLoopRequestFields(
  fields: readonly LoopRequestField[],
  values: Readonly<Record<string, string>>
): LoopRequestPayloadCheck {
  const errors: Record<string, string> = {};
  const payload: Record<string, unknown> = {};
  for (const field of fields) {
    const { value, error } = parseFieldValue(field, values[field.name] ?? "");
    if (error) {
      errors[field.name] = error;
      continue;
    }
    if (value !== undefined) payload[field.name] = value;
  }
  const ok = Object.keys(errors).length === 0;
  return ok ? { ok, errors, payload } : { ok, errors };
}

export function checkLoopRequestJson(text: string, schema: unknown): LoopRequestPayloadCheck {
  const trimmed = text.trim();
  if (trimmed === "") {
    const required = stringList(asRecord(schema)?.required);
    if (required.length === 0) return { ok: true, errors: {}, payload: {} };
    return {
      ok: false,
      errors: Object.fromEntries(required.map(key => [key, `${key} is required.`])),
    };
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(trimmed);
  } catch {
    return { ok: false, errors: { payload: "Payload must be JSON." } };
  }
  const record = asRecord(schema);
  if (record?.type === "object" && asRecord(parsed) === null) {
    return { ok: false, errors: { payload: "Payload must be an object." } };
  }
  const missing = stringList(record?.required).filter(key => {
    const value = asRecord(parsed);
    return value !== null && !Object.hasOwn(value, key);
  });
  if (missing.length > 0) {
    return {
      ok: false,
      errors: Object.fromEntries(missing.map(key => [key, `${key} is required.`])),
    };
  }
  return { ok: true, errors: {}, payload: asRecord(parsed) ?? { value: parsed } };
}

export function loopRequestFieldSeed(
  fields: readonly LoopRequestField[],
  source: unknown
): Record<string, string> {
  const record = asRecord(source) ?? {};
  const seed: Record<string, string> = {};
  for (const field of fields) {
    const value = record[field.name];
    if (value === undefined || value === null) {
      seed[field.name] = "";
      continue;
    }
    seed[field.name] = typeof value === "string" ? value : JSON.stringify(value);
  }
  return seed;
}
