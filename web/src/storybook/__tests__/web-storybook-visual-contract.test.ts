import { describe, expect, it } from "vitest";

// A Visual Contract row with no target is not a missing screenshot — it is a
// state nobody can look at, and without this it surfaces halfway through a
// capture run instead of here.
describe("loop run Visual Contract targets", () => {
  it("stages every VC row against a story export that exists", async () => {
    const { LOOP_RUN_VISUAL_CONTRACT } =
      await import("../../systems/loops/components/stories/loop-run-visual-contract");
    const modules = [
      await import("../../systems/loops/components/stories/loop-run-page.stories"),
      await import("../../systems/loops/components/stories/loop-run-registers.stories"),
      await import("../../systems/loops/components/stories/loop-runs.stories"),
    ];
    const staged = new Set<string>();
    for (const module of modules) {
      const title = (module.default as { title?: string }).title;
      for (const exportName of Object.keys(module)) {
        if (exportName === "default") continue;
        staged.add(`${title}::${exportName}`);
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
