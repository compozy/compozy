import type { LoopTimelineEntry, LoopTimelinePage } from "../types";

/**
 * The durable-to-live seam, as a decision.
 *
 * A run's story arrives in two halves: a durable newest window, then a stream.
 * They only join without a gap if the stream resumes at the exact sequence the
 * window ended on — so the stream must not open until that fence is known.
 *
 * The subtlety worth isolating: `head_seq = 0` is a real fence, not a missing
 * one. A run with no events yet publishes 0, and resuming from 0 is correct.
 * Treating the fence as truthy would leave those runs permanently unsubscribed.
 */
export interface LoopStreamSeam {
  /** The sequence the stream resumes after; absent only while the fence is unknown. */
  afterSequence: number | undefined;
  /** False until the fence exists — opening earlier would drop the interval. */
  ready: boolean;
}

export function loopStreamSeam(headSeq: number | null): LoopStreamSeam {
  return headSeq === null
    ? { afterSequence: undefined, ready: false }
    : { afterSequence: headSeq, ready: true };
}

/**
 * The fence, once known, stays put.
 *
 * A lifecycle frame invalidates the reads, the newest window re-reads at a
 * higher head, and following that head would tear down and re-open the stream
 * on every frame — the busiest runs reconnecting the most. The first fence is
 * the one the subscription was opened on, and it stays correct for as long as
 * that subscription lives; a genuinely new page set (new run, workspace or
 * view) starts over with a new snapshot.
 */
export function latchLoopStreamSeam(latched: number | null, headSeq: number | null): number | null {
  return latched === null ? headSeq : latched;
}

/** The span one entry covers — a single sequence unless the daemon coalesced a run. */
function entrySpanStart(entry: LoopTimelineEntry): number {
  return entry.first_seq ?? entry.seq;
}

/**
 * Entries are flat records of primitives, so a shallow comparison is exact.
 *
 * It deliberately does not stop at reference equality: a refetch can hand back
 * structurally identical entries as fresh objects, and treating those as changed
 * would make the caller set state on every render — a render loop, not a
 * re-render. A re-authored title still differs field by field and still wins.
 */
function sameEntry(left: LoopTimelineEntry, right: LoopTimelineEntry): boolean {
  if (Object.is(left, right)) return true;
  const fields = new Set([...Object.keys(left), ...Object.keys(right)]) as Set<
    keyof LoopTimelineEntry
  >;
  for (const field of fields) {
    if (left[field] !== right[field]) return false;
  }
  return true;
}

function sameEntries(
  left: readonly LoopTimelineEntry[],
  right: readonly LoopTimelineEntry[]
): boolean {
  if (left.length !== right.length) return false;
  return left.every((entry, index) => sameEntry(entry, right[index]));
}

/**
 * One coherent story out of however many reads produced it.
 *
 * Timeline entries are immutable append-only facts keyed by `seq`, so a union
 * across reads can never surface a stale one — which is what makes this a merge
 * rather than a cache workaround.
 *
 * It has to be a union because a refetched page set is not a superset of the one
 * it replaces. A refetch re-reads the newest window unpinned and then re-derives
 * every backward cursor from it, so the whole window slides up by however many
 * events appended — and the oldest pages, the ones somebody paged back to read,
 * fall off the bottom. Holding the union instead means loaded history only ever
 * grows.
 *
 * Two entries can also describe the same heartbeats: a coalesced run that grew
 * republishes its window under a higher `seq` with the same `first_seq`. The
 * containment sweep keeps the wider one and drops what it already covers.
 */
export function mergeTimelineEntries(
  previous: readonly LoopTimelineEntry[],
  incoming: readonly LoopTimelineEntry[]
): readonly LoopTimelineEntry[] {
  const bySeq = new Map<number, LoopTimelineEntry>();
  for (const entry of previous) bySeq.set(entry.seq, entry);
  // A re-read wins over a held copy: the daemon authors the titles, and its
  // newest authoring of a sequence is the one to render.
  for (const entry of incoming) bySeq.set(entry.seq, entry);

  const ordered = [...bySeq.values()].sort((left, right) => right.seq - left.seq);
  const kept: LoopTimelineEntry[] = [];
  let coveredFrom = Number.POSITIVE_INFINITY;
  for (const entry of ordered) {
    if (entry.seq >= coveredFrom) continue;
    kept.push(entry);
    coveredFrom = Math.min(coveredFrom, entrySpanStart(entry));
  }
  // Identity is load-bearing: the caller adjusts state during render, so an
  // unchanged merge has to come back as the very same array.
  return sameEntries(previous, kept) ? previous : kept;
}

/**
 * Everything the page has read of one run's story, plus the fence its stream
 * resumes from. Keyed so a different run, workspace or view starts over instead
 * of inheriting the previous run's beats.
 */
export interface LoopTimelineSnapshot {
  key: string;
  entries: readonly LoopTimelineEntry[];
  fence: number | null;
}

/**
 * Length-prefixed rather than delimited.
 *
 * Any separator character can also occur inside a workspace id or a run id, and
 * then two different runs quietly share a snapshot — one run inheriting the
 * other's beats. Prefixing each part with its length makes the encoding
 * unambiguous whatever the ids contain, and keeps the source plain ASCII.
 */
function keySegment(value: string): string {
  return `${value.length}:${value}`;
}

export function loopTimelineSnapshotKey(workspaceId: string, runId: string, view: string): string {
  return `${keySegment(workspaceId)}${keySegment(runId)}${keySegment(view)}`;
}

export function emptyTimelineSnapshot(key: string): LoopTimelineSnapshot {
  return { key, entries: [], fence: null };
}

/**
 * Folds a fresh page set into the snapshot. Returns the same snapshot when
 * nothing moved, so the caller can compare by identity.
 */
export function advanceTimelineSnapshot(
  snapshot: LoopTimelineSnapshot,
  key: string,
  pages: readonly LoopTimelinePage[]
): LoopTimelineSnapshot {
  const base = snapshot.key === key ? snapshot : emptyTimelineSnapshot(key);
  const entries = mergeTimelineEntries(
    base.entries,
    pages.flatMap(page => page.entries)
  );
  const headSeq = pages.reduce<number | null>(
    (highest, page) => (highest === null ? page.head_seq : Math.max(highest, page.head_seq)),
    null
  );
  const fence = latchLoopStreamSeam(base.fence, headSeq);
  if (base === snapshot && entries === snapshot.entries && fence === snapshot.fence) {
    return snapshot;
  }
  return { key, entries, fence };
}
