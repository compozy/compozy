/**
 * Where each Visual Contract row is staged.
 *
 * `task_05.md` names 36 states the capture run has to photograph. A state with
 * no target is not a missing screenshot — it is a state nobody can look at, and
 * the way that failure usually surfaces is halfway through a capture run. This
 * manifest is the contract between the task's VC table and the stories that
 * satisfy it, and the storybook contract suite fails the moment a row points at
 * a story that is not there.
 *
 * Capture may click a lane to reach a row (the register's lane and selection are
 * in-page state by design), which is why several rows share a story title and
 * differ by the export that drives the click.
 */
export interface LoopRunVisualContractRow {
  /** The `VC-NN` id exactly as `task_05.md` writes it. */
  id: string;
  /** CSF title of the module that stages the state. */
  title: string;
  /** Named story export inside that module. */
  exportName: string;
}

const RUN_PAGE = "systems/loops/components/LoopRunPage";
const REGISTERS = "systems/loops/components/LoopRunRegisters";
const RUNS = "systems/loops/components/LoopRuns";

export const LOOP_RUN_VISUAL_CONTRACT: readonly LoopRunVisualContractRow[] = [
  // S4 — the default read.
  { id: "VC-01", title: RUN_PAGE, exportName: "RegisterRunning" },
  { id: "VC-02", title: REGISTERS, exportName: "DefaultQueued" },
  { id: "VC-03", title: RUN_PAGE, exportName: "Watching" },
  { id: "VC-04", title: RUN_PAGE, exportName: "RegisterDone" },
  { id: "VC-05", title: RUN_PAGE, exportName: "RegisterFailed" },
  { id: "VC-06", title: RUN_PAGE, exportName: "NoOp" },
  { id: "VC-07", title: RUN_PAGE, exportName: "Canceled" },
  { id: "VC-08", title: RUN_PAGE, exportName: "PartialCompletion" },
  { id: "VC-09", title: RUN_PAGE, exportName: "RegisterPrunedArtifact" },
  { id: "VC-10", title: REGISTERS, exportName: "StoryPaging" },
  { id: "VC-11", title: RUN_PAGE, exportName: "ForkLineage" },
  { id: "VC-12", title: REGISTERS, exportName: "DefaultBudgetWarning" },

  // S4 — needs you.
  { id: "VC-13", title: RUN_PAGE, exportName: "RegisterNeedsYou" },
  { id: "VC-14", title: RUN_PAGE, exportName: "RepeatedGenerationRequests" },
  { id: "VC-15", title: REGISTERS, exportName: "NeedsYouExpiry" },
  { id: "VC-16", title: RUN_PAGE, exportName: "RequestAlreadyAnswered" },

  // S5 — the run graph.
  { id: "VC-17", title: REGISTERS, exportName: "GraphLive" },
  { id: "VC-18", title: REGISTERS, exportName: "GraphTerminal" },
  { id: "VC-19", title: REGISTERS, exportName: "GraphFailedAndQuarantined" },
  { id: "VC-20", title: RUN_PAGE, exportName: "RegisterRoutedGraph" },
  { id: "VC-21", title: RUN_PAGE, exportName: "RegisterWideFanOut" },
  { id: "VC-22", title: REGISTERS, exportName: "GraphDeepAndWide" },
  { id: "VC-23", title: REGISTERS, exportName: "NodePanelOpen" },
  { id: "VC-24", title: REGISTERS, exportName: "GraphReducedMotion" },

  // S5 — the roster and the rounds.
  { id: "VC-25", title: REGISTERS, exportName: "RosterComplete" },
  { id: "VC-26", title: REGISTERS, exportName: "RosterMultiAttempt" },
  { id: "VC-27", title: REGISTERS, exportName: "RosterCancelDispositions" },
  { id: "VC-28", title: REGISTERS, exportName: "RosterNoSteps" },
  { id: "VC-29", title: REGISTERS, exportName: "RosterFanOutGrouped" },
  { id: "VC-30", title: REGISTERS, exportName: "GenerationsScoredAndUnscored" },
  { id: "VC-31", title: REGISTERS, exportName: "GenerationsCrashInterrupted" },
  { id: "VC-32", title: REGISTERS, exportName: "NodePanelPrunedSession" },

  // S6 — the runs roster.
  { id: "VC-33", title: RUNS, exportName: "Default" },
  { id: "VC-34", title: RUNS, exportName: "EmptyWorkspace" },
  { id: "VC-35", title: RUNS, exportName: "DozensActive" },
  { id: "VC-36", title: RUNS, exportName: "TransportDegraded" },
];
