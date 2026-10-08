/**
 * What tells sibling child runs apart: the inputs they were started with.
 *
 * A fan-out starts the same loop once per item, so every child carries the same
 * loop name and only its inputs say which batch, region or file it is working
 * on — which is exactly what a reader needs to spot the input that is slow or
 * stuck. Only the inputs that differ between siblings are shown; a lone child
 * shows the few it was given.
 */

/** How many inputs a row names before the rest stay behind the tooltip. */
const CHILD_INPUT_LIMIT = 3;
/** A value longer than this is cut; the tooltip carries the whole line. */
const CHILD_INPUT_VALUE_MAX = 28;

export interface LoopChildRunInputs {
  /** "batch: api · region: us-east" — the distinguishing inputs, shortened. */
  label: string;
  /** Every distinguishing input in full, for the tooltip. */
  title: string;
}

function stringifyInput(value: unknown): string {
  if (typeof value === "string") return value;
  if (value === null || value === undefined) return String(value);
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  return JSON.stringify(value) ?? "";
}

function shorten(value: string): string {
  return value.length > CHILD_INPUT_VALUE_MAX
    ? `${value.slice(0, CHILD_INPUT_VALUE_MAX - 1)}…`
    : value;
}

/** Keys whose value is not the same across every sibling that has been read. */
function distinguishingKeys(siblings: readonly Record<string, unknown>[]): string[] {
  const keys = [...new Set(siblings.flatMap(inputs => Object.keys(inputs)))].sort();
  if (siblings.length < 2) return keys;
  return keys.filter(key => {
    const values = new Set(siblings.map(inputs => stringifyInput(inputs[key])));
    return values.size > 1;
  });
}

/**
 * One label per child, keyed by run id. A child whose inputs are not read yet,
 * or whose siblings were all started identically, gets no label.
 */
export function childRunInputLabels(
  children: readonly { runId: string; inputs: Record<string, unknown> | null | undefined }[]
): Map<string, LoopChildRunInputs> {
  const read = children.filter(
    (child): child is { runId: string; inputs: Record<string, unknown> } =>
      child.inputs !== null && child.inputs !== undefined
  );
  const keys = distinguishingKeys(read.map(child => child.inputs));
  const labels = new Map<string, LoopChildRunInputs>();
  if (keys.length === 0) return labels;
  for (const child of read) {
    const present = keys.filter(key => key in child.inputs);
    if (present.length === 0) continue;
    const pairs = present.map(key => [key, stringifyInput(child.inputs[key])] as const);
    labels.set(child.runId, {
      label: pairs
        .slice(0, CHILD_INPUT_LIMIT)
        .map(([key, value]) => `${key}: ${shorten(value)}`)
        .join(" · "),
      title: pairs.map(([key, value]) => `${key}: ${value}`).join("\n"),
    });
  }
  return labels;
}
