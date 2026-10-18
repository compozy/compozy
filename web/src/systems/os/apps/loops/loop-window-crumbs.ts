import type { TopbarCrumb, TopbarSlotValue } from "@compozy/ui";

export type LoopRunsTrailOptions =
  | {
      level: "runs";
      openLoops: () => void;
      onBack: () => void;
    }
  | {
      level: "run";
      openLoops: () => void;
      openRuns: () => void;
      openLoop?: () => void;
      loopName?: string;
      runId: string;
      /** Human label for the run leaf; falls back to the run id while the run loads. */
      runLabel?: string;
      onBack: () => void;
    }
  | {
      level: "compare";
      openLoops: () => void;
      openRuns: () => void;
      openLoop?: () => void;
      openRun: () => void;
      loopName?: string;
      runId: string;
      runLabel?: string;
      onBack: () => void;
    };

/**
 * Window-local drill-in trail for the Loops run area.
 * List: Loops › Runs. Detail: Loops › Runs › {loopName}? › {runLabel ?? runId}.
 * Compare: Loops › Runs › {loopName}? › {runLabel ?? runId} › Compare.
 */
export function loopRunsTrail(
  options: LoopRunsTrailOptions
): Pick<TopbarSlotValue, "crumb" | "crumbs" | "onBack"> {
  const loops: TopbarCrumb = {
    id: "loops",
    label: "Loops",
    onSelect: options.openLoops,
  };

  if (options.level === "runs") {
    return { crumb: "Runs", crumbs: [loops], onBack: options.onBack };
  }

  const runs: TopbarCrumb = {
    id: "runs",
    label: "Runs",
    onSelect: options.openRuns,
  };
  const loop = loopCrumb(options.loopName, options.openLoop);
  const parents = loop === undefined ? [loops, runs] : [loops, runs, loop];
  const runLabel =
    options.runLabel === undefined || options.runLabel === "" ? options.runId : options.runLabel;

  if (options.level === "run") {
    return { crumb: runLabel, crumbs: parents, onBack: options.onBack };
  }

  return {
    crumb: "Compare",
    crumbs: [...parents, { id: "run", label: runLabel, onSelect: options.openRun }],
    onBack: options.onBack,
  };
}

function loopCrumb(
  loopName: string | undefined,
  openLoop: (() => void) | undefined
): TopbarCrumb | undefined {
  if (loopName === undefined || loopName === "" || openLoop === undefined) {
    return undefined;
  }
  return { id: "loop", label: loopName, onSelect: openLoop };
}
