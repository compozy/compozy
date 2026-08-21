import type { LoopTimelineFilter } from "../types";

export type LoopTimelineView = NonNullable<LoopTimelineFilter["view"]>;

/** The timeline views accepted by the HTTP contract. */
export const LOOP_TIMELINE_VIEWS = [
  "notable",
  "all",
] as const satisfies readonly LoopTimelineView[];

/** Narrows raw URL or test input to the generated timeline-view contract. */
export function isLoopTimelineView(value: string): value is LoopTimelineView {
  return LOOP_TIMELINE_VIEWS.some(view => view === value);
}
