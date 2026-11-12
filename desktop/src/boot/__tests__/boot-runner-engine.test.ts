import { describe, expect, it } from "vitest";

import { RUNNER, RUNNER_AGENTS, RunnerEngine } from "../page/runner-engine";

/** A repeatable sequence so obstacle layouts are the same on every run. */
function seeded(seed: number): () => number {
  let state = seed;
  return () => {
    state = (state * 1_103_515_245 + 12_345) % 2_147_483_648;
    return state / 2_147_483_648;
  };
}

function run(engine: RunnerEngine, seconds: number, start = 0): number {
  let now = start;
  for (let frame = 0; frame < seconds * 60; frame += 1) {
    now += 1000 / 60;
    engine.step(1 / 60, now);
  }
  return now;
}

// Invariant: the waiting game plays itself without a miss, waits at rest when it starts paused,
// and a person's miss shows Failed then hands the run to the next always-on agent.
// Owner: boot page waiting game. Canonical suite: boot-runner-engine.test.ts.
describe("boot runner engine", () => {
  it.each([1, 7, 42])("Should clear every stack on autopilot (seed %i)", seed => {
    const engine = new RunnerEngine({ autoplay: true, random: seeded(seed) });
    run(engine, 90);
    expect(engine.mood).toBe("working");
    expect(engine.agent).toBe(0);
    expect(engine.score).toBeGreaterThan(30);
  });

  it("Should rest until a press starts a paused run", () => {
    const engine = new RunnerEngine({ autoplay: false, random: seeded(3) });
    run(engine, 5);
    expect(engine).toMatchObject({ mood: "paused", running: false, travelled: 0 });
    engine.press(5000);
    expect(engine).toMatchObject({ mood: "working", running: true });
    expect(engine.velocity).toBe(RUNNER.jump);
  });

  it("Should show Failed on a miss and hand the run to the next agent", () => {
    const engine = new RunnerEngine({ autoplay: true, random: seeded(11) });
    // One press takes control; without another, the runner meets the next stack.
    engine.press(0);
    let now = 0;
    while (engine.mood !== "failed" && now < 6000) now = run(engine, 0.1, now);
    expect(engine.mood).toBe("failed");
    run(engine, RUNNER.handoffMs / 1000 + 0.1, now);
    expect(engine).toMatchObject({ mood: "working", score: 0, agent: 1 });
    expect(RUNNER_AGENTS[engine.agent]).toBe("qa-runner");
  });
});
