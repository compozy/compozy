import { describe, expect, it } from "vitest";

import { LOOP_RUN_VISUAL_CONTRACT } from "../../systems/loops/components/stories/loop-run-visual-contract";
import loopRunPageSource from "../../systems/loops/components/stories/loop-run-page.stories.tsx?raw";
import loopRunRegistersSource from "../../systems/loops/components/stories/loop-run-registers.stories.tsx?raw";
import loopRunsSource from "../../systems/loops/components/stories/loop-runs.stories.tsx?raw";

// A Visual Contract row with no target is not a missing screenshot — it is a
// state nobody can look at, and without this it surfaces halfway through a
// capture run instead of here.
describe("loop run Visual Contract targets", () => {
  it("stages every VC row against a story export that exists", () => {
    const sources = [loopRunPageSource, loopRunRegistersSource, loopRunsSource];
    const staged = new Set<string>();
    for (const source of sources) {
      const title = source.match(/title:\s*"([^"]+)"/)?.[1];
      if (!title) throw new Error("Story source is missing its CSF title");
      for (const match of source.matchAll(/^export const (\w+): Story =/gm)) {
        staged.add(`${title}::${match[1]}`);
      }
    }

    const missing = LOOP_RUN_VISUAL_CONTRACT.filter(
      row => !staged.has(`${row.title}::${row.exportName}`)
    ).map(row => `${row.id} → ${row.title}::${row.exportName}`);

    expect(missing).toEqual([]);
    // The table in `task_05.md` is VC-01..VC-36 exactly, in order. Counting rows
    // and de-duplicating ids would still accept VC-37 standing in for VC-36.
    const expectedIds = Array.from(
      { length: 36 },
      (_, index) => `VC-${String(index + 1).padStart(2, "0")}`
    );
    expect(LOOP_RUN_VISUAL_CONTRACT.map(row => row.id)).toEqual(expectedIds);
  });
});
