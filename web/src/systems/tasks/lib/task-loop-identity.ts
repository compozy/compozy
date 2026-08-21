import type { TaskListItem } from "../types";

/**
 * Structured Loop provenance projected by the daemon on both the catalog item
 * and the single-task read. The web never parses task ids to recover it
 * (ADR-004: classification rides provenance columns, never id strings).
 */
export type TaskLoopProvenance = NonNullable<TaskListItem["loop"]>;

/** Separator for plain-words identity: U+00B7 with a space either side. */
const IDENTITY_SEPARATOR = " · ";

/**
 * Grouped-entity labels from the locked vocabulary
 * (`docs/design/opendesign/loop-legibility/DESIGN-NOTES.md`): the plain register
 * says "step", never "loop cell", and names the parent "Loop run", never
 * "Loop coordinator".
 */
const TASK_LOOP_ROLE_LABELS: Record<TaskLoopProvenance["role"], string> = {
  coordinator: "Loop run",
  cell: "Loop step",
};

export function taskLoopRoleLabel(loop: TaskLoopProvenance): string {
  return TASK_LOOP_ROLE_LABELS[loop.role];
}

/**
 * `loop_name` is omitted exactly when the owning run was retention-deleted and
 * is unrecoverable, so its absence is the run-availability signal — the record
 * still renders, but it carries no link (US-002.EC-2).
 */
/**
 * What the list and the detail both say when retention removed the run.
 *
 * One constant, because these two renderers describe the same fact and an
 * operator reading both should not meet two wordings for it.
 */
export const RUN_GONE_LABEL = "Run no longer available";

export function taskLoopRunAvailable(loop: TaskLoopProvenance): boolean {
  return Boolean(loop.loop_name?.trim());
}

/**
 * Plain-words identity for a revealed record — "revisao-paralela · run" for the
 * parent, "revisao-paralela · round 1 · step revisor-perf" for a step. A machine
 * id is never the primary text (US-002.AC-1); when the run is gone the record
 * leads with its generic entity name and the id carries identity in secondary
 * text instead.
 *
 * Fan-out items past the first append their index so three workers of the same
 * step never render as three identical rows.
 */
export function taskLoopIdentityLabel(loop: TaskLoopProvenance): string {
  const name = loop.loop_name?.trim();
  if (!name) {
    return taskLoopRoleLabel(loop);
  }
  if (loop.role === "coordinator") {
    return [name, "run"].join(IDENTITY_SEPARATOR);
  }
  const segments = [name];
  if (typeof loop.generation === "number") {
    segments.push(`round ${loop.generation}`);
  }
  if (loop.node_id?.trim()) {
    segments.push(`step ${loop.node_id.trim()}`);
  }
  if (typeof loop.item_index === "number" && loop.item_index > 0) {
    segments.push(`item ${loop.item_index}`);
  }
  return segments.join(IDENTITY_SEPARATOR);
}

export interface TaskLoopRunLink {
  to: "/loop-runs/$runId";
  params: { runId: string };
}

/**
 * Deep link into the run page — the observability home for loop work (ADR-002).
 * Returns `null` for a retention-deleted run so callers render the truthful
 * degrade rather than a broken link.
 */
export function taskLoopRunLink(loop: TaskLoopProvenance): TaskLoopRunLink | null {
  if (!taskLoopRunAvailable(loop)) {
    return null;
  }
  return { to: "/loop-runs/$runId", params: { runId: loop.run_id } };
}
