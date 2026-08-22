import type {
  LoopDefinition,
  LoopNodeControl,
  LoopNodeWait,
  LoopRequest,
  LoopBriefing,
  LoopFanoutRollup,
  LoopRosterNode,
  LoopRunEventFrame,
  LoopRunGeneration,
  LoopRunRecord,
  LoopTimelineEntry,
} from "../../types";

/**
 * One run-page story scenario: the run projection, its pinned definition, the
 * replayed SSE frames, and the durable lifecycle payloads exactly as
 * `getLoopRun` returns them. Everything a story renders derives from these — no
 * story assembles a view model by hand.
 */
export interface LoopRunStoryScenario {
  run: LoopRunRecord;
  definition: LoopDefinition;
  frames: LoopRunEventFrame[];
  generations: LoopRunGeneration[];
  /** Durable per-node control truth, exactly as `getLoopRun` returns it. */
  nodeControls?: LoopNodeControl[];
  /** Durable wait cells, exactly as `getLoopRun` returns them. */
  waits?: LoopNodeWait[];

  requests?: LoopRequest[];

  /**
   * The served verdict (ADR-005), and the one read no scenario may skip.
   *
   * It was optional, and the projection filled the gap with a manufactured
   * briefing. That made a story with no staged verdict render a plausible strip
   * anyway — which is how a visual-contract row came to photograph a state
   * nobody had staged and pass. Build it with `briefingFor`, which copies the
   * run's own server-owned fields so the two reads cannot disagree.
   */
  briefing: LoopBriefing;
  /**
   * The roster and timeline reads stay optional: a scenario that omits them
   * renders those registers empty rather than inventing rows it never described.
   */
  rosterNodes?: LoopRosterNode[];
  rosterRollups?: LoopFanoutRollup[];
  timeline?: LoopTimelineEntry[];
  /**
   * How much of `timeline` the first page holds.
   *
   * Set it and the scenario stages a run whose history does not fit in one read:
   * the story renders the newest window with `Load earlier beats` live, and the
   * control pages backward through the rest. Without it the whole timeline is
   * the first page, which is what every short scenario means.
   */
  timelinePageSize?: number;
}
