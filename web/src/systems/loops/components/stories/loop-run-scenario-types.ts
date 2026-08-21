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
  LoopWatchEventsState,
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
  watchEvents?: LoopWatchEventsState;
  /** Durable per-node control truth, exactly as `getLoopRun` returns it. */
  nodeControls?: LoopNodeControl[];
  /** Durable wait cells, exactly as `getLoopRun` returns them. */
  waits?: LoopNodeWait[];

  requests?: LoopRequest[];

  /**
   * The three run reads (ADR-005). Optional so a scenario can stage only the
   * default read; a scenario that omits them renders the registers empty rather
   * than inventing a roster the fixture never described.
   */
  briefing?: LoopBriefing;
  rosterNodes?: LoopRosterNode[];
  rosterRollups?: LoopFanoutRollup[];
  timeline?: LoopTimelineEntry[];
}
