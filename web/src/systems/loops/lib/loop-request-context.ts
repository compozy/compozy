export interface LoopRequestContextEntry {
  key: string;
  value: string;
}

/** One readable context block: key/value rows for an object, or a plain line otherwise. */
export interface LoopRequestContextBlock {
  entries: LoopRequestContextEntry[];
  text: string | null;
}

/**
 * Projects a request's context value onto a displayable block. Objects become
 * key/value rows; scalars and arrays become one printable line; an empty
 * object or an absent/empty value has nothing to show and yields `null`.
 */
export function loopRequestContextBlock(value: unknown): LoopRequestContextBlock | null {
  const entries = contextEntries(value);
  if (entries.length > 0) return { entries, text: null };
  const text = contextText(value);
  return text === null ? null : { entries, text };
}

function contextEntries(value: unknown): LoopRequestContextEntry[] {
  if (typeof value !== "object" || value === null || Array.isArray(value)) return [];
  return Object.entries(value as Record<string, unknown>).map(([key, entry]) => ({
    key,
    value: printableValue(entry),
  }));
}

function contextText(value: unknown): string | null {
  if (value === undefined || value === null) return null;
  if (typeof value === "object" && !Array.isArray(value)) return null;
  const text = printableValue(value);
  return text === "" ? null : text;
}

function printableValue(value: unknown): string {
  if (typeof value === "string") return value;
  if (value === undefined) return "";
  try {
    return JSON.stringify(value) ?? String(value);
  } catch {
    return String(value);
  }
}
