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
import type { LoopBriefing, LoopRunEventFrame, LoopTimelineEntry } from "../../types";

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

/**
 * The newest window of a staged timeline, and whether anything is behind it.
 *
 * A scenario that declares a page size is describing a run whose history does
 * not fit in one read — which is the whole subject of VC-10 and E2E-015. The
 * story then renders exactly what the daemon's first page would contain, and
 * `Load earlier beats` is live because there genuinely is an earlier page.
 */
function stageTimeline(scenario: LoopRunStoryScenario): {
  timeline: LoopTimelineEntry[];
  hasOlder: boolean;
} {
  const timeline = scenario.timeline ?? [];
  const pageSize = scenario.timelinePageSize;
  if (pageSize === undefined || timeline.length <= pageSize) {
    return { timeline, hasOlder: false };
  }
  return { timeline: timeline.slice(0, pageSize), hasOlder: true };
}

/**
 * The briefing a scenario that predates the read layer still has a right to.
 *
 * Before the two-register redesign a page story only stated `run`, `definition`,
 * `frames` and `generations`. Defaulting the briefing to `null` left every one
 * of those stories rendering a page with no verdict strip and no progress — a
 * state target that no longer targets anything, while the storybook contract
 * test only checks that the export exists.
 *
 * This derives nothing: `status` and `progress` are both server-owned fields the
 * run record already carries. What it cannot recover is the daemon's written
 * headline, the blockers and the artifacts, so it says so rather than inventing
 * them, and the roster and timeline stay empty because the fixture genuinely
 * never described them.
 */
function legacyBriefing(run: LoopRunStoryScenario["run"]): LoopBriefing {
  return {
    run_id: run.id,
    status: run.status,
    tone: "ok",
    headline: `${run.loop_name} · round ${run.progress.round}`,
    detail: "This story predates the run read layer, so only the run's own fields are staged.",
    blockers: [],
    artifacts: [],
    progress: run.progress,
    usage: { tokens: run.tokens_used },
  };
}

export function buildScenarioProps(scenario: LoopRunStoryScenario): ScenarioBodyProps {
  const { run, definition, generations } = scenario;
  const staged = stageTimeline(scenario);
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
      briefing: scenario.briefing ?? legacyBriefing(run),
      nodes: scenario.rosterNodes ?? [],
      rollups: scenario.rosterRollups ?? [],
      timeline: staged.timeline,
      graph: view.graph,
    }),
    storyPaging: {
      hasOlder: staged.hasOlder,
      isLoading: false,
      isError: false,
      isLoadingOlder: false,
      // A capture is a still frame; the control has to be present and enabled
      // for the contract, and the interactive backfill belongs to the play step.
      onLoadOlder: () => undefined,
    },
    rosterNodes: scenario.rosterNodes ?? [],
    rosterRollups: scenario.rosterRollups ?? [],
    // Pinned, not the wall clock: an elapsed reading that moves between captures
    // turns every visual-contract diff into noise.
    nowMs: STORY_NOW,
    onDecision: () => undefined,
  };
}
