import type { operations } from "../../src/generated/compozy-openapi";

export type LoopRunNodesContract =
  operations["getLoopRunNodes"]["responses"][200]["content"]["application/json"];
export type LoopBriefingContract =
  operations["getLoopRunBriefing"]["responses"][200]["content"]["application/json"];
export type LoopTimelineContract =
  operations["getLoopRunTimeline"]["responses"][200]["content"]["application/json"];

export function assertMonotonicLoopTimeline(entries: LoopTimelineContract["entries"]): void {
  for (let index = 1; index < entries.length; index += 1) {
    const previous = entries[index - 1];
    const current = entries[index];
    if (!previous || !current || previous.seq <= current.seq) {
      throw new Error(`Loop timeline is not newest-first at index ${index}`);
    }
  }
}

export function assertLoopReadContractParity(
  nodes: LoopRunNodesContract,
  briefing: LoopBriefingContract,
  timeline: LoopTimelineContract
): void {
  if (nodes.run_id !== briefing.run_id || nodes.run_id !== timeline.run_id) {
    throw new Error("Loop read projections do not belong to the same run");
  }
  if (timeline.head_seq < 0) {
    throw new Error("Loop timeline head_seq must be nonnegative");
  }
  assertMonotonicLoopTimeline(timeline.entries);
}
