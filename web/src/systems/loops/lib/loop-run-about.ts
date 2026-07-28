import type { LoopDefinition, LoopRunRecord } from "../types";

/**
 * The About-rail projections (redesign spec §3/§4): the watched subject line,
 * the per-input rows, and the humanized start origin. Everything reads from
 * `run.inputs` + the pinned definition — generic over any loop, never
 * special-cased by loop name (§5.1).
 */

export interface LoopRunInputRow {
  key: string;
  label: string;
  value: string;
  /** Declared `ref.kind === "agent"` inputs render with an avatar seed. */
  isAgent: boolean;
}

type DeclaredInputs = NonNullable<LoopDefinition["inputs"]>;

function isScalar(value: unknown): value is string | number | boolean {
  return typeof value === "string" || typeof value === "number" || typeof value === "boolean";
}

/** `pr` → "PR" (short keys read as initialisms); `max_files` → "Max files". */
export function humanizeInputKey(key: string): string {
  const spaced = key.replace(/[_-]+/g, " ").trim();
  if (spaced.length <= 3 && !spaced.includes(" ")) return spaced.toUpperCase();
  return spaced.charAt(0).toUpperCase() + spaced.slice(1);
}

function declaredRefKind(declared: DeclaredInputs | undefined, key: string): string {
  return declared?.[key]?.ref?.kind ?? "";
}

function scalarEntries(run: LoopRunRecord): [string, string | number | boolean][] {
  return Object.entries(run.inputs ?? {})
    .filter((entry): entry is [string, string | number | boolean] => isScalar(entry[1]))
    .sort(([a], [b]) => a.localeCompare(b));
}

/**
 * The loop-declared primary input rendered `key value` (`PR 128`) — required
 * declared scalars win, then any declared scalar, then the lexicographically
 * first scalar input (entries are key-sorted for stable About rows).
 */
export function watchedSubject(
  run: LoopRunRecord,
  definition: Pick<LoopDefinition, "inputs"> | undefined
): string | null {
  const entries = scalarEntries(run);
  if (entries.length === 0) return null;
  const declared = definition?.inputs;
  const required = entries.find(([key]) => declared?.[key]?.required === true);
  const declaredEntry = entries.find(([key]) => declared?.[key] !== undefined);
  const [key, value] = required ?? declaredEntry ?? entries[0];
  return `${humanizeInputKey(key)} ${String(value)}`;
}

/** One About row per scalar run input, agent-bound inputs flagged for an avatar. */
export function buildInputRows(
  run: LoopRunRecord,
  definition: Pick<LoopDefinition, "inputs"> | undefined
): LoopRunInputRow[] {
  return scalarEntries(run).map(([key, value]) => ({
    key,
    label: humanizeInputKey(key),
    value: String(value),
    isAgent: declaredRefKind(definition?.inputs, key) === "agent",
  }));
}

const START_ORIGIN_LABELS: Record<string, string> = {
  manual: "Started by hand",
  cli: "The CLI",
  http: "An API call",
  uds: "A local socket call",
  native_tool: "An agent tool call",
  schedule: "A schedule",
  webhook: "A webhook",
  event: "An event",
};

/** Humanized `started_origin_kind/ref` ("A schedule · nightly"); raw kind fallback. */
export function humanizeStartOrigin(run: LoopRunRecord): string {
  const kind = run.started_origin_kind?.trim() ?? "";
  const ref = run.started_origin_ref?.trim() ?? "";
  const label = START_ORIGIN_LABELS[kind] ?? (kind || "Started by hand");
  return ref ? `${label} · ${ref}` : label;
}
