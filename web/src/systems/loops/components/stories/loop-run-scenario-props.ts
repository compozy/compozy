import {
  applyLoopEventFrame,
  emptyLoopRunLiveState,
  type LoopRunLiveState,
} from "../../lib/loop-events";
import { projectLoopRunPageView } from "../../lib/loop-run-page-view";
import { projectLoopRunRegisters } from "../../lib/loop-run-registers-view";
import { materializeContractFixture } from "../../mocks/materialize-contract-fixture";
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

/**
 * Everything a scenario can state as data. Whatever describes where the reader
 * currently is — the disclosure, the open node, what retention has since removed
 * — belongs to the story component, exactly as it belongs to the page.
 */
export type ScenarioBodyProps = Omit<
  LoopRunPageBodyProps,
  "inspect" | "nodeSelection" | "onNodeSelectionChange" | "prunedSessionIds"
>;

export function buildScenarioProps(scenario: LoopRunStoryScenario): ScenarioBodyProps {
  const { run, definition, generations } = scenario;
  const live = reduceLiveState(scenario.frames);
  const {
    effectiveRun,
    elapsedLabel: _elapsedLabel,
    ...view
  } = projectLoopRunPageView({
    run,
    generations,
    live,
    definition,
    nowMs: STORY_NOW,
    nodeControls: scenario.nodeControls,
    waits: scenario.waits,
    requests: scenario.requests,
  });
  return {
    ...view,
    run: effectiveRun,
    materializedContract: materializeContractFixture(definition.contract, run.inputs ?? {}),
    generations,
    workspaceLabel: "Home",
    versionLabel: `v${run.definition_version} · pinned`,
    // Both registers come from the same three reads the live page uses, so a
    // captured story is evidence about the real projection, not a hand-built one.
    registers: projectLoopRunRegisters({
      briefing: scenario.briefing ?? null,
      nodes: scenario.rosterNodes ?? [],
      rollups: scenario.rosterRollups ?? [],
      timeline: scenario.timeline ?? [],
      graph: view.graph,
    }),
    rosterNodes: scenario.rosterNodes ?? [],
    rosterRollups: scenario.rosterRollups ?? [],
    // Pinned, not the wall clock: an elapsed reading that moves between captures
    // turns every visual-contract diff into noise.
    nowMs: STORY_NOW,
    onDecision: () => undefined,
  };
}
