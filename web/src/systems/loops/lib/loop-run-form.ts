import type { LoopInputSchema, LoopInputSchemaField } from "../types";

/** The resolved run-form input values, keyed by declared input name. */
export type LoopRunInputs = Record<string, unknown>;

/**
 * Seeds the run-form inputs from each declared input's default. A boolean with no
 * default seeds `false` (its switch has a concrete off state); every other type
 * seeds only when the schema carries a default, so a required text/agent/ref field
 * starts empty and keeps Run disabled until the operator fills it.
 */
export function initialRunInputs(schema?: LoopInputSchema): LoopRunInputs {
  const inputs: LoopRunInputs = {};
  if (!schema) return inputs;
  for (const [name, field] of Object.entries(schema)) {
    if (field.default !== undefined && field.default !== null) {
      inputs[name] = field.default;
    } else if (field.type === "boolean") {
      inputs[name] = false;
    }
  }
  return inputs;
}

/**
 * True when a declared input carries a usable value for its type: a real boolean,
 * a finite number, or a non-blank string/ref/file/agent handle. Blank text never
 * satisfies a required marker (truthful validation — the daemon rejects it too).
 */
export function hasInputValue(field: LoopInputSchemaField, value: unknown): boolean {
  if (field.type === "boolean") return typeof value === "boolean";
  if (field.type === "number") return typeof value === "number" && Number.isFinite(value);
  if (typeof value === "string") return value.trim() !== "";
  return value !== undefined && value !== null;
}

/** The declared required inputs still missing a value; Run stays disabled while non-empty. */
export function missingRequiredInputs(
  schema: LoopInputSchema | undefined,
  inputs: LoopRunInputs
): string[] {
  if (!schema) return [];
  const missing: string[] = [];
  for (const [name, field] of Object.entries(schema)) {
    if (field.required && !hasInputValue(field, inputs[name])) missing.push(name);
  }
  return missing;
}

/** The run form is submittable once every required input is filled. */
export function isRunFormValid(
  schema: LoopInputSchema | undefined,
  inputs: LoopRunInputs
): boolean {
  return missingRequiredInputs(schema, inputs).length === 0;
}

/**
 * Builds the `inputs` request body: every defined value passes through (an explicit
 * empty string is kept only when the field's default is `""`, the "disable this
 * check" convention, §4.3). `undefined`/`null` are dropped so the daemon applies
 * the declared default instead of receiving a null.
 */
export function serializeRunInputs(
  schema: LoopInputSchema | undefined,
  inputs: LoopRunInputs
): LoopRunInputs {
  const out: LoopRunInputs = {};
  for (const [name, value] of Object.entries(inputs)) {
    if (value === undefined || value === null) continue;
    if (typeof value === "string" && value.trim() === "") {
      if (schema?.[name]?.default === "") out[name] = "";
      continue;
    }
    out[name] = value;
  }
  return out;
}
