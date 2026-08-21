import { buildStoryBeats } from "../lib/loop-run-story-beats";
import type { LoopStoryBeat } from "../lib/loop-run-story-beats";
import { useLoopRunTimeline } from "./use-loop-run-reads";

/** What the Events lane needs, minus the `view` the component states itself. */
export interface LoopRunEventsReadState {
  beats: readonly LoopStoryBeat[];
  hasOlder: boolean;
  isLoading: boolean;
  isError: boolean;
  isLoadingOlder: boolean;
  onLoadOlder: () => void;
}

/**
 * The Events lane's own read: the raw activity union, at `view=all`.
 *
 * Separate from the Story's `notable` read on purpose. They are two different
 * questions over one durable timeline, they page independently, and the daemon
 * keys its cursors to `{run, view, …}` — so sharing one query would make a
 * backward page in one lane silently move the other.
 *
 * **This read never drives the stream.** The subscription's fence comes from the
 * `notable` read alone (`loopStreamSeam(timelineRead.headSeq)` on the page); a
 * second seam would open a second subscription, and two subscriptions on one run
 * is how a page starts double-counting its own events. The latch this read keeps
 * internally is scoped to its own snapshot key and is deliberately unread.
 *
 * `enabled` is the operator opening Inspect. The raw union is the largest read
 * on the page and nobody has asked for it until the lane that shows it exists.
 */
export function useLoopRunEventsRead(
  workspaceId: string,
  runId: string,
  enabled: boolean
): LoopRunEventsReadState {
  const read = useLoopRunTimeline(workspaceId, runId, "all", enabled);
  return {
    beats: buildStoryBeats(read.entries),
    hasOlder: enabled && read.hasOlder,
    // A query that was never enabled is pending, not loading. Reporting "reading
    // this run's events…" for a read nobody started would be the lane inventing
    // activity, which is the one thing the escape hatch must never do.
    isLoading: enabled && read.isLoading,
    isError: enabled && read.isError,
    isLoadingOlder: enabled && read.isLoadingOlder,
    onLoadOlder: read.loadOlder,
  };
}
