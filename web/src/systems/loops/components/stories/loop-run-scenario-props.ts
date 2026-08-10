import {
  applyLoopEventFrame,
  emptyLoopRunLiveState,
  type LoopRunLiveState,
} from "../../lib/loop-events";
import { buildNodeNowLines } from "../../lib/loop-node-now-view";
import { projectLoopRunPageView } from "../../lib/loop-run-page-view";
import type { LoopRunPageBodyProps } from "../run-page/loop-run-page-body";
import type { LoopRunStoryScenario } from "./loop-run-scenario-types";
import { STORY_NOW } from "./loop-run-page-fixture-world";
import type { LoopRunEventFrame } from "../../types";

/**
 * The one place a story scenario becomes run-page props. It runs the fixture's
 * frames through the production reducer and the production view projection, so
 * a Storybook capture is evidence about the real derivation path rather than a
 * hand-assembled view model that could drift from it.
 */

/** Reduces the scenario's frames through the production reducer, like the page. */
function reduceLiveState(frames: readonly LoopRunEventFrame[]): LoopRunLiveState {
  return frames.reduce(applyLoopEventFrame, emptyLoopRunLiveState());
}

export type ScenarioBodyProps = Omit<LoopRunPageBodyProps, "inspect">;

export function buildScenarioProps(scenario: LoopRunStoryScenario): ScenarioBodyProps {
  const { run, definition, generations, watchEvents } = scenario;
  const live = reduceLiveState(scenario.frames);
  const { effectiveRun, ...view } = projectLoopRunPageView({
    run,
    generations,
    live,
    definition,
    nowMs: STORY_NOW,
    nodeControls: scenario.nodeControls,
    waits: scenario.waits,
  });
  return {
    ...view,
    run: effectiveRun,
    definition,
    materializedContract: definition.contract,
    goalTurns: scenario.goalTurns ?? [],
    generations,
    watchEvents,
    frames: live.frames,
    workspaceLabel: "Home",
    versionLabel: `v${run.definition_version} · pinned`,
    nodeNowLines: buildNodeNowLines(view.nodeLifecycles, view.graph, live.retrySchedules),
    nodesById: new Map(view.nodeLifecycles.map(node => [node.nodeId, node])),
    onDecision: () => undefined,
    onStartNewRun: () => undefined,
  };
}
