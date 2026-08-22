import { isTerminalLoopStatus } from "../../lib/loop-formatters";
import { loopRunFixtures } from "../../mocks/fixtures";
import type { LoopRun } from "../../types";

/**
 * The roster at the scale a real workspace reaches.
 *
 * `rr-scale` is about one thing: "scale changes the count, not the composition —
 * the needs-you group still sorts first and stays small; active collapses into
 * one table." The dozens therefore have to be *active*.
 *
 * Cycling every seed does the opposite. `loopRunFixtures` is mostly terminal
 * runs, so thirty runs drawn from it in order inherit that mix and the roster
 * sorts them into a needs-you group of two, an active group of eight and a
 * recent group of twenty — the dozens land in Recent, and the one group the
 * contract is about stays small.
 *
 * So the seeds are partitioned by the same two facts `groupOf` groups on
 * (`lib/loop-runs-view.ts`): a served `attention` summary, and whether the
 * status is terminal. Reusing the roster's own predicate rather than naming ids
 * means a change to the grouping rule moves this fixture with it, instead of
 * silently landing the bulk in the wrong group again.
 */

/** Total runs staged, as `task_05.md` pins for this row. */
const TOTAL_RUNS = 30;
/** One run waiting on a person: enough to lead the roster, small by design. */
const NEEDS_YOU_RUNS = 1;
/** A tail of settled runs, so the composition still reads as three groups. */
const RECENT_RUNS = 3;

const needsYouSeeds = loopRunFixtures.filter(run => Boolean(run.attention));
const activeSeeds = loopRunFixtures.filter(
  run => !run.attention && !isTerminalLoopStatus(run.status)
);
const recentSeeds = loopRunFixtures.filter(
  run => !run.attention && isTerminalLoopStatus(run.status)
);

/**
 * `LoopRunRow` keys by run id, so every copy of a seed needs its own.
 *
 * The index is global rather than per-group so no two rows can collide even when
 * two groups draw from seeds that share a name.
 */
function cycle(seeds: readonly LoopRun[], count: number, offset: number): LoopRun[] {
  return Array.from({ length: count }, (_unused, index) => {
    const seed = seeds[index % seeds.length] as LoopRun;
    return { ...seed, id: `${seed.id}-${offset + index + 1}` };
  });
}

/**
 * Thirty runs: one needs-you, twenty-six active, three settled.
 *
 * Server order is preserved by the roster — it partitions, it never sorts — so
 * the order here is the order the groups render.
 */
export const dozensActiveRuns: readonly LoopRun[] = [
  ...cycle(needsYouSeeds, NEEDS_YOU_RUNS, 0),
  ...cycle(activeSeeds, TOTAL_RUNS - NEEDS_YOU_RUNS - RECENT_RUNS, NEEDS_YOU_RUNS),
  ...cycle(recentSeeds, RECENT_RUNS, TOTAL_RUNS - RECENT_RUNS),
];
