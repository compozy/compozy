// Suite: Loop run-area window trail
// Invariant: every run-area level keeps Loops and Runs as labeled exits; loop name is a parent only when known.
// Boundary IN: trail level, loop name, and navigation callbacks.
// Boundary OUT: Topbar slot crumb / crumbs / onBack — rendering is owned by Topbar.

import { describe, expect, it, vi } from "vitest";

import { loopRunsTrail } from "../loop-window-crumbs";

describe("loopRunsTrail", () => {
  const openLoops = vi.fn();
  const openRuns = vi.fn();
  const openLoop = vi.fn();
  const openRun = vi.fn();
  const onBack = vi.fn();

  it("Should publish Loops as the only parent when the leaf is Runs", () => {
    const trail = loopRunsTrail({ level: "runs", openLoops, onBack });

    expect(trail.crumb).toBe("Runs");
    expect(trail.onBack).toBe(onBack);
    expect(trail.crumbs).toEqual([{ id: "loops", label: "Loops", onSelect: openLoops }]);
  });

  it("Should keep Runs in the trail when the loop name is not known yet", () => {
    const trail = loopRunsTrail({
      level: "run",
      onBack,
      openLoops,
      openRuns,
      runId: "looprun_running",
    });

    expect(trail.crumb).toBe("looprun_running");
    expect(crumbIds(trail.crumbs)).toEqual(["loops", "runs"]);
  });

  it("Should append the loop name without dropping Runs", () => {
    const trail = loopRunsTrail({
      level: "run",
      loopName: "implement-tasks",
      onBack,
      openLoop,
      openLoops,
      openRuns,
      runId: "looprun_running",
    });

    expect(trail.crumb).toBe("looprun_running");
    expect(crumbIds(trail.crumbs)).toEqual(["loops", "runs", "loop"]);
    expect(trail.crumbs?.[2]).toEqual({
      id: "loop",
      label: "implement-tasks",
      onSelect: openLoop,
    });
  });

  it("Should keep Runs and the run id as parents on the compare leaf", () => {
    const trail = loopRunsTrail({
      level: "compare",
      loopName: "implement-tasks",
      onBack,
      openLoop,
      openLoops,
      openRun,
      openRuns,
      runId: "looprun_running",
    });

    expect(trail.crumb).toBe("Compare");
    expect(crumbIds(trail.crumbs)).toEqual(["loops", "runs", "loop", "run"]);
    expect(trail.crumbs?.[3]).toEqual({
      id: "run",
      label: "looprun_running",
      onSelect: openRun,
    });
  });
});

function crumbIds(crumbs: readonly { id: string }[] | undefined): string[] {
  return (crumbs ?? []).map(crumb => crumb.id);
}
