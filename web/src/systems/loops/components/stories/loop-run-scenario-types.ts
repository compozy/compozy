import type { GoalTurnTimelineItem } from "../../hooks/use-goal-turns";
import type {
  LoopDefinition,
  LoopNodeControl,
  LoopNodeWait,
  LoopRequest,
  LoopRunEventFrame,
  LoopRunGeneration,
  LoopRunRecord,
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
  goalTurns?: GoalTurnTimelineItem[];
  /** Durable per-node control truth, exactly as `getLoopRun` returns it. */
  nodeControls?: LoopNodeControl[];
  /** Durable wait cells, exactly as `getLoopRun` returns them. */
  waits?: LoopNodeWait[];

  requests?: LoopRequest[];
}
