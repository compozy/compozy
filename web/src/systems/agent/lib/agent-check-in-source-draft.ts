/** The Add source dialog's draft: fields read from a kind's parameter schema, and the entry it writes. */
import type { AgentHeartbeatSourceCatalog } from "../types";
import type { CheckInSource } from "./agent-check-in-sources";

type Catalog = AgentHeartbeatSourceCatalog;
type Kind = Catalog["kinds"][number];

/** The parameter shapes the form can edit; anything else stays as written in the file. */
export type SourceParamType = "string" | "list" | "boolean" | "duration";

export interface SourceParamField {
  key: string;
  label: string;
  type: SourceParamType;
  description?: string;
  required: boolean;
  /** A string pattern from the schema, checked before saving. */
  pattern?: string;
}

export type SourceParamValue = string | string[] | boolean;

export interface SourceDraft {
  id: string;
  /** `kind:<extension>/<kind>` or `script:<name>`; empty until one is picked. */
  ref: string;
  /** Values for the kind's fields, by key. */
  params: Record<string, SourceParamValue>;
  /** A script's `with`, as name and text pairs. */
  pairs: SourceValuePair[];
  /** `with` values the form can't show, written back as they were. */
  kept: Record<string, unknown>;
}

/** One name and text value of a script's `with`; `uid` keeps its row while names change. */
export interface SourceValuePair {
  uid: string;
  key: string;
  value: string;
}

export function sourceValuePair(key = "", value = ""): SourceValuePair {
  return { uid: crypto.randomUUID(), key, value };
}

export type SourceDraftErrors = Partial<Record<string, string>>;

/** Rule 1 of the spec: a source's id. */
export const SOURCE_ID_PATTERN = /^[a-z][a-z0-9-]{0,39}$/;
const DURATION_PATTERN = /^(0|(\d+(\.\d+)?(ns|us|µs|ms|s|m|h))+)$/;
const MAX_WITH_BYTES = 4096;

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function labelOf(key: string) {
  const words = key.replace(/[_-]+/g, " ").trim();
  return words ? words[0]!.toUpperCase() + words.slice(1) : key;
}

function fieldType(schema: Record<string, unknown>): SourceParamType | null {
  if (schema.type === "boolean") return "boolean";
  if (schema.type === "string") return schema.format === "duration" ? "duration" : "string";
  if (schema.type === "array" && isRecord(schema.items) && schema.items.type === "string") {
    return "list";
  }
  return null;
}

/** The kind's parameters the form can edit, in schema order. */
export function sourceParamFields(schema: unknown): SourceParamField[] {
  if (!isRecord(schema) || !isRecord(schema.properties)) return [];
  const required = new Set(Array.isArray(schema.required) ? schema.required : []);
  const fields: SourceParamField[] = [];
  for (const [key, property] of Object.entries(schema.properties)) {
    if (!isRecord(property)) continue;
    const type = fieldType(property);
    if (!type) continue;
    fields.push({
      key,
      label: typeof property.title === "string" && property.title ? property.title : labelOf(key),
      type,
      description: typeof property.description === "string" ? property.description : undefined,
      required: required.has(key),
      pattern:
        type === "string" && typeof property.pattern === "string" ? property.pattern : undefined,
    });
  }
  return fields;
}

export function kindOf(catalog: Catalog | null, ref: string): Kind | undefined {
  if (!ref.startsWith("kind:")) return undefined;
  const kind = ref.slice("kind:".length);
  return catalog?.kinds.find(item => item.kind === kind);
}

export function sourceRef(source: Pick<CheckInSource, "kind" | "script">) {
  if (source.script) return `script:${source.script}`;
  return source.kind ? `kind:${source.kind}` : "";
}

function fieldValue(field: SourceParamField, value: unknown): SourceParamValue | undefined {
  if (field.type === "boolean") return typeof value === "boolean" ? value : undefined;
  if (field.type === "list") {
    return Array.isArray(value) && value.every(item => typeof item === "string")
      ? [...(value as string[])]
      : undefined;
  }
  return typeof value === "string" ? value : undefined;
}

/** A new name that isn't taken yet: `issues`, then `issues-2`. */
export function suggestSourceId(ref: string, others: readonly string[]) {
  const taken = new Set(others);
  const name =
    ref
      .slice(ref.indexOf(":") + 1)
      .split("/")
      .pop() ?? "";
  const base =
    name
      .toLowerCase()
      .replace(/[^a-z0-9-]+/g, "-")
      .replace(/^[^a-z]+/, "")
      .replace(/-+$/, "")
      .slice(0, 36) || "source";
  if (!taken.has(base)) return base;
  let n = 2;
  while (taken.has(`${base}-${n}`)) n += 1;
  return `${base}-${n}`;
}

export function blankSourceDraft(): SourceDraft {
  return { id: "", ref: "", params: {}, pairs: [], kept: {} };
}

/** The draft for an existing entry; values the form can't show are kept for the save. */
export function sourceDraftFrom(source: CheckInSource, catalog: Catalog | null): SourceDraft {
  const ref = sourceRef(source);
  const values = isRecord(source.with) ? source.with : {};
  const draft: SourceDraft = { ...blankSourceDraft(), id: source.id, ref };
  if (source.script) {
    for (const [key, value] of Object.entries(values)) {
      if (typeof value === "string") draft.pairs.push(sourceValuePair(key, value));
      else draft.kept[key] = value;
    }
    return draft;
  }
  const fields = sourceParamFields(kindOf(catalog, ref)?.params_schema);
  for (const [key, value] of Object.entries(values)) {
    const field = fields.find(item => item.key === key);
    const shown = field ? fieldValue(field, value) : undefined;
    if (shown === undefined) draft.kept[key] = value;
    else draft.params[key] = shown;
  }
  return draft;
}

function draftWith(draft: SourceDraft, fields: readonly SourceParamField[]) {
  const values: Record<string, unknown> = {};
  if (draft.ref.startsWith("script:")) {
    for (const pair of draft.pairs) {
      if (pair.key.trim() !== "") values[pair.key.trim()] = pair.value;
    }
  } else {
    for (const field of fields) {
      const value = draft.params[field.key];
      if (value === undefined || value === "" || (Array.isArray(value) && value.length === 0)) {
        continue;
      }
      if (field.type === "boolean" && value === false && !(field.key in draft.kept)) {
        // An untouched switch writes nothing, so the kind's own default applies.
        continue;
      }
      values[field.key] = typeof value === "string" ? value.trim() : value;
    }
  }
  for (const [key, value] of Object.entries(draft.kept)) {
    if (!(key in values)) values[key] = value;
  }
  return values;
}

/** The `context.sources` entry the draft saves; `with` is left out when empty. */
export function sourceFromDraft(draft: SourceDraft, catalog: Catalog | null): CheckInSource {
  const fields = sourceParamFields(kindOf(catalog, draft.ref)?.params_schema);
  const values = draftWith(draft, fields);
  const [type, name] = [
    draft.ref.slice(0, draft.ref.indexOf(":")),
    draft.ref.slice(draft.ref.indexOf(":") + 1),
  ];
  const entry: CheckInSource = { id: draft.id.trim() };
  if (type === "script") entry.script = name;
  else entry.kind = name;
  if (Object.keys(values).length > 0) entry.with = values as CheckInSource["with"];
  return entry;
}

function patternError(field: SourceParamField, value: string) {
  if (!field.pattern) return undefined;
  try {
    return new RegExp(field.pattern, "u").test(value) ? undefined : "This doesn't look right.";
  } catch {
    return undefined;
  }
}

function fieldError(field: SourceParamField, value: SourceParamValue | undefined) {
  const text = typeof value === "string" ? value.trim() : "";
  const empty =
    value === undefined || (text === "" && !Array.isArray(value) && typeof value !== "boolean");
  if (field.required && (empty || (Array.isArray(value) && value.length === 0))) {
    return "Fill this in.";
  }
  if (empty || typeof value !== "string") return undefined;
  if (field.type === "duration" && !DURATION_PATTERN.test(text)) {
    return "Use a length of time, like 30m or 24h.";
  }
  return patternError(field, text);
}

/**
 * What stops the draft from saving, by field: `id`, `ref`, `param:<key>`, `pairs` and `with`.
 * `others` are the ids the file already uses besides this one.
 */
export function validateSourceDraft(
  draft: SourceDraft,
  catalog: Catalog | null,
  others: readonly string[]
): SourceDraftErrors {
  const errors: SourceDraftErrors = {};
  const id = draft.id.trim();
  if (id === "") errors.id = "Give it a name.";
  else if (!SOURCE_ID_PATTERN.test(id)) {
    errors.id = "Use lowercase letters, numbers and dashes, starting with a letter.";
  } else if (others.includes(id)) errors.id = "Another source already has this name.";
  if (draft.ref === "") errors.ref = "Pick what it reads.";
  if (draft.ref.startsWith("script:")) {
    const keys = draft.pairs.map(pair => pair.key.trim());
    if (keys.some(key => key === "")) errors.pairs = "Give each value a name.";
    else if (new Set(keys).size !== keys.length) errors.pairs = "Each name can be used once.";
  }
  const fields = sourceParamFields(kindOf(catalog, draft.ref)?.params_schema);
  for (const field of fields) {
    const error = fieldError(field, draft.params[field.key]);
    if (error) errors[`param:${field.key}`] = error;
  }
  const size = new TextEncoder().encode(JSON.stringify(draftWith(draft, fields))).length;
  if (size > MAX_WITH_BYTES) errors.with = "Its settings are too long. Keep them under 4 KB.";
  return errors;
}

/** The kinds and scripts the dialog offers; scripts only when the daemon runs them. */
export function sourceChoices(catalog: Catalog) {
  return {
    kinds: catalog.kinds,
    scripts: catalog.scripts_enabled ? catalog.scripts : [],
    empty: catalog.kinds.length === 0 && (!catalog.scripts_enabled || catalog.scripts.length === 0),
  };
}

/** Add stays offered until the file holds as many sources as the daemon allows. */
export function canAddSource(count: number, catalog: Catalog | null) {
  return catalog === null || count < catalog.limits.max_per_agent;
}
