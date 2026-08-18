export type LoopRequestFieldControl =
  | { kind: "text" }
  | { kind: "number" }
  | { kind: "integer" }
  | { kind: "boolean" }
  | { kind: "json" }
  | { kind: "select"; options: readonly LoopRequestSelectOption[] };

export interface LoopRequestSelectOption {
  token: string;
  label: string;
  value: unknown;
}

export interface LoopRequestField {
  name: string;
  required: boolean;
  description: string;
  control: LoopRequestFieldControl;
}

export interface LoopRequestPayloadCheck {
  ok: boolean;

  errors: Record<string, string>;

  payload?: Record<string, unknown>;
}

function asRecord(value: unknown): Record<string, unknown> | null {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;
}

function stringList(value: unknown): string[] {
  if (!Array.isArray(value)) return [];
  return value.filter((entry): entry is string => typeof entry === "string");
}

function optionLabel(value: unknown): string {
  if (typeof value === "string") return value === "" ? '""' : value;
  return JSON.stringify(value) ?? String(value);
}

function selectControl(values: readonly unknown[]): LoopRequestFieldControl {
  return {
    kind: "select",
    options: values.map((value, index) => ({
      token: `option-${index}`,
      label: optionLabel(value),
      value,
    })),
  };
}

function fieldControl(schema: Record<string, unknown>): LoopRequestFieldControl {
  if (Array.isArray(schema.enum)) {
    return selectControl(schema.enum);
  }
  if (Object.hasOwn(schema, "const")) {
    return selectControl([schema.const]);
  }
  if (schema.oneOf !== undefined || schema.anyOf !== undefined || schema.allOf !== undefined) {
    return { kind: "json" };
  }
  switch (schema.type) {
    case "string":
      return { kind: "text" };
    case "number":
      return { kind: "number" };
    case "integer":
      return { kind: "integer" };
    case "boolean":
      return { kind: "boolean" };
    default:
      return { kind: "json" };
  }
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
      required: required.has(name),
      description: typeof property.description === "string" ? property.description : "",
      control: fieldControl(property),
    };
  });
}

export function isLoopRequestFieldSchema(schema: unknown): boolean {
  return loopRequestFields(schema).length > 0;
}

/** Human label for a schema field: "migration_url" / "dryRun" → "Migration url" / "Dry run". */
export function loopRequestFieldLabel(field: Pick<LoopRequestField, "name">): string {
  const words = field.name
    .replace(/([a-z0-9])([A-Z])/g, "$1 $2")
    .replace(/[_-]+/g, " ")
    .trim();
  if (words === "") return field.name;
  return words.charAt(0).toUpperCase() + words.slice(1).toLowerCase();
}

function parseFieldValue(
  field: LoopRequestField,
  raw: string
): { value?: unknown; error?: string } {
  const label = loopRequestFieldLabel(field);
  const trimmed = raw.trim();
  if (trimmed === "") {
    return field.required ? { error: `${label} is required.` } : {};
  }
  switch (field.control.kind) {
    case "select": {
      const option = field.control.options.find(candidate => candidate.token === raw);
      return option ? { value: option.value } : { error: `${label} must be an offered value.` };
    }
    case "boolean":
      if (trimmed === "true") return { value: true };
      if (trimmed === "false") return { value: false };
      return { error: `${label} must be Yes or No.` };
    case "number":
    case "integer": {
      const parsed = Number(trimmed);
      if (!Number.isFinite(parsed)) return { error: `${label} must be a number.` };
      if (field.control.kind === "integer" && !Number.isInteger(parsed)) {
        return { error: `${label} must be a whole number.` };
      }
      return { value: parsed };
    }
    case "text":
      return { value: trimmed };
    default:
      try {
        return { value: JSON.parse(trimmed) as unknown };
      } catch {
        return { error: `${label} must be JSON.` };
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
    if (value === undefined) {
      seed[field.name] = "";
      continue;
    }
    if (field.control.kind === "select") {
      seed[field.name] =
        field.control.options.find(option => jsonValueEqual(option.value, value))?.token ?? "";
      continue;
    }
    seed[field.name] = typeof value === "string" ? value : (JSON.stringify(value) ?? "");
  }
  return seed;
}

export function loopRequestFieldKind(field: LoopRequestField): string {
  if (field.control.kind === "text") return "string";
  if (field.control.kind === "select") return "enum";
  return field.control.kind;
}

function jsonValueEqual(left: unknown, right: unknown): boolean {
  if (Object.is(left, right)) return true;
  if (Array.isArray(left) && Array.isArray(right)) {
    return (
      left.length === right.length &&
      left.every((value, index) => jsonValueEqual(value, right[index]))
    );
  }
  const leftRecord = asRecord(left);
  const rightRecord = asRecord(right);
  if (!leftRecord || !rightRecord) return false;
  const leftKeys = Object.keys(leftRecord);
  const rightKeys = Object.keys(rightRecord);
  return (
    leftKeys.length === rightKeys.length &&
    leftKeys.every(
      key => Object.hasOwn(rightRecord, key) && jsonValueEqual(leftRecord[key], rightRecord[key])
    )
  );
}
