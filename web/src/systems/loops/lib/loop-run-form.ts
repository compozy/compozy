import type { NetworkParticipationDraft } from "@/lib/network-participation";

import type { LoopEnvironmentSpec, LoopInputSchema, LoopInputSchemaField } from "../types";
import { LOOP_ENVIRONMENT_MODE_LABELS } from "./loop-node-schema-types";

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

/** Folded Inputs takeaway: required vs optional counts from the declared schema. */
export function declaredInputCountsGist(schema?: LoopInputSchema): string {
  const names = schema ? Object.keys(schema) : [];
  const required = names.filter(name => schema?.[name]?.required).length;
  return `${required} required · ${names.length - required} optional`;
}

/** Folded Participation takeaway from the live draft. */
export function participationGist(draft: NetworkParticipationDraft): string {
  if (draft.mode === "local") return "Local";
  return `Live · ${draft.channelStrategy}`;
}

/** Folded Environment takeaway from the per-run override (inherit = loop default). */
export function environmentGist(value: LoopEnvironmentSpec | null): string {
  if (!value) return "Loop default";
  if (value.mode === "worktree") {
    const ref = value.worktree_ref?.trim();
    return ref ? `worktree · ${ref}` : "worktree";
  }
  if (value.mode === "directory") {
    const path = value.directory?.trim();
    return path ? `directory · ${path}` : "directory";
  }
  return LOOP_ENVIRONMENT_MODE_LABELS[value.mode];
}
