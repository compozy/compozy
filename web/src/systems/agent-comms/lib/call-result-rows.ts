/**
 * Schema-aware rendering of a typed result.
 *
 * A call's result is a JSON object shaped by its contract, so the useful reading
 * is a flat path → value list rather than a pretty-printed blob: `verdict`,
 * `findings[0].note`. Paths are stable, copyable, and line up with the contract
 * the operator wrote.
 *
 * Everything here operates on `result_preview`, which is *already bounded by the
 * daemon*. This module never claims the preview is the whole payload — when it
 * is not, the caller offers the full fetch and says so. Two separate bounds keep
 * a pathological result from freezing the pane: a row cap, and an array cap that
 * summarizes instead of enumerating.
 */

/** Rows past this are not rendered; the caller offers the full payload instead. */
const MAX_ROWS = 60;

/** Array elements past this are summarized rather than listed. */
const MAX_ARRAY_ITEMS = 3;

export interface CallResultRow {
  /** Dotted/bracketed path into the result object. */
  path: string;
  /** JSON-encoded scalar, so a string reads as `"…"` and a number as `88`. */
  value: string;
  /** True when this row stands for elided siblings rather than one value. */
  summary: boolean;
}

export type CallResultShape =
  | { kind: "rows"; rows: CallResultRow[]; truncated: boolean }
  /** A non-object root — valid JSON, just not the object shape contracts use. */
  | { kind: "scalar"; value: string }
  /** The daemon sent no preview: a resultless terminal, or nothing yet. */
  | { kind: "absent" };

function encodeScalar(value: unknown): string {
  if (value === undefined) return "undefined";
  try {
    return JSON.stringify(value) ?? String(value);
  } catch {
    // Cyclic or otherwise unserializable — say so rather than throwing inside a
    // render path.
    return "[unserializable]";
  }
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

interface FlattenState {
  rows: CallResultRow[];
  truncated: boolean;
}

function appendRow(state: FlattenState, row: CallResultRow): boolean {
  if (state.rows.length >= MAX_ROWS) {
    state.truncated = true;
    return false;
  }
  state.rows.push(row);
  return true;
}

function flatten(value: unknown, path: string, state: FlattenState): void {
  if (state.rows.length >= MAX_ROWS) {
    state.truncated = true;
    return;
  }

  if (Array.isArray(value)) {
    const shown = Math.min(value.length, MAX_ARRAY_ITEMS);
    for (let index = 0; index < shown; index += 1) {
      flatten(value[index], `${path}[${index}]`, state);
    }
    if (value.length > shown) {
      appendRow(state, {
        path: `${path}[0..${value.length - 1}]`,
        value: `${value.length} records · first ${shown} shown in preview`,
        summary: true,
      });
    }
    return;
  }

  if (isPlainObject(value)) {
    for (const [key, child] of Object.entries(value)) {
      flatten(child, path === "" ? key : `${path}.${key}`, state);
      if (state.truncated) return;
    }
    return;
  }

  appendRow(state, { path, value: encodeScalar(value), summary: false });
}

/**
 * Project a result preview into renderable rows.
 *
 * `null` is deliberately treated as a scalar rather than as absence: a contract
 * may legitimately admit a null, and collapsing that into "no result" would
 * misreport a completed call as a silent one.
 */
export function buildCallResultShape(preview: unknown): CallResultShape {
  if (preview === undefined) return { kind: "absent" };
  if (!isPlainObject(preview) && !Array.isArray(preview)) {
    return { kind: "scalar", value: encodeScalar(preview) };
  }
  if (Object.keys(preview).length === 0) {
    return { kind: "scalar", value: encodeScalar(preview) };
  }
  const state: FlattenState = { rows: [], truncated: false };
  flatten(preview, "", state);
  return { kind: "rows", rows: state.rows, truncated: state.truncated };
}
