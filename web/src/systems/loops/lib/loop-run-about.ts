import type { LoopDefinition, LoopRunRecord } from "../types";

export interface LoopRunInputRow {
  key: string;
  label: string;
  value: string;
  /** Declared `type: agent` inputs render with an avatar seed. */
  isAgent: boolean;
}

function isScalar(value: unknown): value is string | number | boolean {
  return typeof value === "string" || typeof value === "number" || typeof value === "boolean";
}

/** `pr` → "PR" (short keys read as initialisms); `max_files` → "Max files". */
export function humanizeInputKey(key: string): string {
  const spaced = key.replace(/[_-]+/g, " ").trim();
  if (spaced.length <= 3 && !spaced.includes(" ")) return spaced.toUpperCase();
  return spaced.charAt(0).toUpperCase() + spaced.slice(1);
}

function scalarEntries(run: LoopRunRecord): [string, string | number | boolean][] {
  return Object.entries(run.inputs ?? {})
    .filter((entry): entry is [string, string | number | boolean] => isScalar(entry[1]))
    .sort(([a], [b]) => a.localeCompare(b));
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
    isAgent: definition?.inputs?.[key]?.type === "agent",
  }));
}

const START_ORIGIN_LABELS: Record<string, string> = {
  manual: "hand",
  cli: "The CLI",
  http: "An API call",
  uds: "A local socket call",
  native_tool: "An agent tool call",
  schedule: "A schedule",
  webhook: "A webhook",
  event: "An event",
};

/** Unprefixed origin/actor (`hand`, `A schedule · nightly`) for the About rail. */
export function humanizeStartOrigin(run: LoopRunRecord): string {
  const kind = run.started_origin_kind?.trim() ?? "";
  const ref = run.started_origin_ref?.trim() ?? "";
  const label = START_ORIGIN_LABELS[kind] ?? (kind || "hand");
  return ref ? `${label} · ${ref}` : label;
}
