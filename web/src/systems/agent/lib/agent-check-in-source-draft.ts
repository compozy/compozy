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
  /** A switch's schema default: what the kind does while the file leaves the key out. */
  default?: boolean;
}

export type SourceParamValue = string | string[] | boolean;

export interface SourceDraft {
  id: string;
  /** `kind:<extension>/<kind>` or `script:<name>`; empty until one is picked. */
  ref: string;
  /** Values for the kind's fields, by key; a key is absent until the file or the operator sets it. */
  params: Record<string, SourceParamValue>;
  /** The kind's field values as the file wrote them, so an unchanged one saves exactly as read. */
  read: Record<string, SourceParamValue>;
  /** A script's `with`, as name and text pairs. */
  pairs: SourceValuePair[];
  /** `with` values the form can't show, written back as they were. */
  kept: Record<string, unknown>;
  /** The file wrote a `with`, so an empty one stays rather than vanishing. */
  hasWith: boolean;
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
      default:
        type === "boolean" && typeof property.default === "boolean" ? property.default : undefined,
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
  const kept: Record<string, unknown> = Object.create(null);
  return { id: "", ref: "", params: {}, read: {}, pairs: [], kept, hasWith: false };
}

/** The draft for an existing entry; values the form can't show are kept for the save. */
export function sourceDraftFrom(source: CheckInSource, catalog: Catalog | null): SourceDraft {
  const ref = sourceRef(source);
  const values = isRecord(source.with) ? source.with : {};
  const draft: SourceDraft = {
    ...blankSourceDraft(),
    id: source.id,
    ref,
    hasWith: isRecord(source.with),
  };
  if (source.script) {
    for (const [key, value] of Object.entries(values)) {
      if (typeof value === "string") draft.pairs.push(sourceValuePair(key, value));
      else draft.kept[key] = value;
    }
    return draft;
  }
  const fields = new Map(
    sourceParamFields(kindOf(catalog, ref)?.params_schema).map(field => [field.key, field])
  );
  for (const [key, value] of Object.entries(values)) {
    const field = fields.get(key);
    const shown = field ? fieldValue(field, value) : undefined;
    if (shown === undefined) draft.kept[key] = value;
    else {
      draft.params[key] = shown;
      draft.read[key] = Array.isArray(shown) ? [...shown] : shown;
    }
  }
  return draft;
}

/** What a switch shows: its value, or the kind's default while the file leaves it out. */
export function sourceSwitchOn(field: SourceParamField, value: SourceParamValue | undefined) {
  return (typeof value === "boolean" ? value : field.default) === true;
}

function sameValue(a: SourceParamValue, b: SourceParamValue) {
  if (Array.isArray(a) && Array.isArray(b)) {
    return a.length === b.length && a.every((item, index) => item === b[index]);
  }
  return a === b;
}

/**
 * A field's value for the file, or `undefined` to leave the key out. A value the file had and
 * the operator didn't change saves as read, `false`, `""` and `[]` included. A key the file left
 * out stays out until the operator sets it, except a required switch, which writes what it shows; a
 * switch they touched is written even at its default. Clearing text or emptying a list removes it.
 */
function fieldWith(draft: SourceDraft, field: SourceParamField) {
  const value = draft.params[field.key];
  if (value === undefined) {
    return field.required && field.type === "boolean" ? sourceSwitchOn(field, value) : undefined;
  }
  const read = draft.read[field.key];
  if (read !== undefined && sameValue(value, read)) return value;
  if (typeof value === "string") return value.trim() === "" ? undefined : value.trim();
  if (Array.isArray(value) && value.length === 0) return undefined;
  return value;
}

function draftWith(draft: SourceDraft, fields: readonly SourceParamField[]) {
  // No prototype: a setting named `constructor` or `__proto__` is kept like any other.
  const values: Record<string, unknown> = Object.create(null);
  if (draft.ref.startsWith("script:")) {
    for (const pair of draft.pairs) {
      if (pair.key.trim() !== "") values[pair.key.trim()] = pair.value;
    }
  } else {
    for (const field of fields) {
      const value = fieldWith(draft, field);
      if (value !== undefined) values[field.key] = value;
    }
  }
  for (const [key, value] of Object.entries(draft.kept)) {
    if (!Object.hasOwn(values, key)) values[key] = value;
  }
  return values;
}

/** The `context.sources` entry the draft saves; an empty `with` only stays when the file had one. */
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
  if (Object.keys(values).length > 0 || draft.hasWith) {
    entry.with = values as CheckInSource["with"];
  }
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
  // A switch always shows a value, and the save writes it when the kind requires one.
  if (field.type === "boolean") return undefined;
  const text = typeof value === "string" ? value.trim() : "";
  const empty = value === undefined || (text === "" && !Array.isArray(value));
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
    // A name the file already uses for a value the form can't show would replace it unseen.
    const taken = keys.some(key => Object.hasOwn(draft.kept, key));
    if (keys.some(key => key === "")) errors.pairs = "Give each value a name.";
    else if (taken || new Set(keys).size !== keys.length) {
      errors.pairs = "Each name can be used once.";
    }
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
