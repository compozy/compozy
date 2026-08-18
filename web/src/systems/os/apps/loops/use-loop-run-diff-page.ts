import { useNavigate } from "@tanstack/react-router";

import { useCurrentWindowLiveDataEnabled } from "../../hooks/use-window-live-data-enabled";
import {
  comparableGenerations,
  type LoopDiffView,
  type LoopRun,
  type LoopRunDiffRouteSearch,
  type LoopRunRecord,
  LoopTimetravelError,
  loopRunDiffQuery,
  projectLoopDiff,
  useLoopRun,
  useLoopRunDiff,
  useLoopRuns,
} from "@/systems/loops";

const SIBLING_RUN_LIMIT = 50;

export type LoopRunDiffMode = "generation" | "run";

export interface LoopRunDiffRunOption {
  id: string;
  label: string;
}

export interface LoopRunDiffPageModel {
  againstGeneration: number | null;
  againstRunId: string;
  baseGeneration: number | null;

  diffError?: string;
  diffView: LoopDiffView | null;
  generations: readonly number[];
  goToLoop: () => void;
  goToLoops: () => void;
  goToRun: () => void;

  hasComparison: boolean;
  isDiffLoading: boolean;
  isRunLoading: boolean;
  loopName: string;
  mode: LoopRunDiffMode;
  onAgainstGenerationChange: (generation: number) => void;
  onAgainstRunChange: (runId: string) => void;
  onBaseGenerationChange: (generation: number) => void;
  onModeChange: (mode: LoopRunDiffMode) => void;
  run?: LoopRunRecord;
  runError: Error | null;
  runOptions: LoopRunDiffRunOption[];
}

export function useLoopRunDiffPage(
  workspaceId: string,
  runId: string,
  search: LoopRunDiffRouteSearch
): LoopRunDiffPageModel {
  const navigate = useNavigate();
  const liveDataEnabled = useCurrentWindowLiveDataEnabled();
  const runQuery = useLoopRun(workspaceId, runId, liveDataEnabled);
  const run = runQuery.data?.run;
  const loopName = run?.loop_name ?? "";
  const generations = comparableGenerations(runQuery.data?.generations ?? []);

  const siblingsQuery = useLoopRuns(
    workspaceId,
    { limit: SIBLING_RUN_LIMIT, loop: loopName },
    liveDataEnabled && loopName !== ""
  );
  const runOptions = siblingRunOptions(siblingsQuery.data?.runs, runId);

  const baseGeneration = search.generation ?? generations[0] ?? run?.generation ?? null;
  const query = loopRunDiffQuery(search);
  const diffQuery = useLoopRunDiff(
    workspaceId,
    runId,
    query ?? {},
    query !== null && liveDataEnabled
  );

  const applySearch = (patch: LoopRunDiffRouteSearch) => {
    void navigate({
      params: { runId },
      replace: true,
      search: { ...search, ...patch },
      to: "/loop-runs/$runId/diff",
    });
  };

  const diff = query === null ? undefined : diffQuery.data;
  return {
    againstGeneration: search.against_generation ?? null,
    againstRunId: search.against_run ?? "",
    baseGeneration,
    diffError: query === null ? undefined : diffRefusal(diffQuery.error),
    diffView: diff ? projectLoopDiff(diff) : null,
    generations,
    goToLoop: () => {
      void navigate({ params: { name: loopName }, to: "/loops/$name" });
    },
    goToLoops: () => {
      void navigate({ to: "/loops" });
    },
    goToRun: () => {
      void navigate({ params: { runId }, to: "/loop-runs/$runId" });
    },
    hasComparison: query !== null,
    isDiffLoading: query !== null && diffQuery.isLoading,
    isRunLoading: runQuery.isLoading,
    loopName,
    mode: search.against_run === undefined ? "generation" : "run",
    onAgainstGenerationChange: generation => {
      applySearch({
        against_generation: generation,
        against_run: undefined,
        generation: baseGeneration ?? undefined,
      });
    },
    onAgainstRunChange: againstRunId => {
      applySearch({
        against_generation: undefined,
        against_run: againstRunId || undefined,
        generation: baseGeneration ?? undefined,
      });
    },
    onBaseGenerationChange: generation => {
      applySearch({ generation });
    },
    onModeChange: mode => {
      applySearch(
        mode === "run"
          ? {
              against_generation: undefined,
              against_run: runOptions[0]?.id,
              generation: baseGeneration ?? undefined,
            }
          : {
              against_generation: otherGeneration(generations, baseGeneration),
              against_run: undefined,
              generation: baseGeneration ?? undefined,
            }
      );
    },
    run,
    runError: runQuery.error,
    runOptions,
  };
}

function siblingRunOptions(
  runs: readonly LoopRun[] | undefined,
  runId: string
): LoopRunDiffRunOption[] {
  const options: LoopRunDiffRunOption[] = [];
  for (const entry of runs ?? []) {
    if (entry.id === runId) continue;
    options.push({ id: entry.id, label: `${entry.id} · generation ${entry.generation}` });
  }
  return options;
}

function diffRefusal(error: Error | null): string | undefined {
  if (!error) {
    return undefined;
  }
  return error instanceof LoopTimetravelError && error.code !== ""
    ? `${error.code} · ${error.message}`
    : error.message;
}

function otherGeneration(
  generations: readonly number[],
  baseGeneration: number | null
): number | undefined {
  return generations.find(generation => generation !== baseGeneration);
}
