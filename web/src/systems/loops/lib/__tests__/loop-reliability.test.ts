import { describe, expect, it } from "vitest";

import { readLoopGraph } from "../loop-graph";
import { buildLoopReliability, formatLoopDuration } from "../loop-reliability";
import { heroEffectiveLifecycle } from "../../mocks/fixture-hero-path";
import { loopDetailByName } from "../../mocks/fixtures";
import type { LoopDefinitionGraph } from "../../types";

const delivery = loopDetailByName.get("software-delivery")!;
const deliveryGraph = readLoopGraph(delivery.definition);

function graphOf(nodes: ReadonlyArray<Record<string, unknown>>) {
  return readLoopGraph({ graph: { nodes, edges: [] } as unknown as LoopDefinitionGraph });
}

describe("loop-reliability", () => {
  it("Should state the retry posture from the effective lifecycle envelope", () => {
    const tiles = buildLoopReliability({
      effectiveLifecycle: heroEffectiveLifecycle,
      graph: deliveryGraph,
    });
    const retry = tiles.find(tile => tile.id === "retry");
    expect(retry?.body).toContain("Up to 3 attempts");
    expect(retry?.body).toContain("from 30s to 10m");
    expect(retry?.codes).toEqual(["retry", "backoff", "quarantine"]);
  });

  it("Should omit the retry tile when the daemon reports no lifecycle envelope", () => {
    const tiles = buildLoopReliability({ effectiveLifecycle: null, graph: deliveryGraph });
    expect(tiles.map(tile => tile.id)).not.toContain("retry");
  });

  it("Should state the time budget only from authored timeout and deadline keys", () => {
    const authored = buildLoopReliability({
      effectiveLifecycle: heroEffectiveLifecycle,
      graph: deliveryGraph,
    });
    const deadline = authored.find(tile => tile.id === "deadline");
    expect(deadline?.body).toContain("One attempt stops at 45m");
    expect(deadline?.body).toContain("the whole step at 2h");
    expect(deadline?.codes).toEqual(["timeout", "deadline"]);

    const unauthored = buildLoopReliability({
      effectiveLifecycle: heroEffectiveLifecycle,
      graph: graphOf([{ id: "run", class: "action", kind: "run-agent" }]),
    });
    expect(unauthored.map(tile => tile.id)).not.toContain("deadline");
  });

  it("Should describe waits only for a loop that declares a wait node", () => {
    const withWait = buildLoopReliability({
      effectiveLifecycle: heroEffectiveLifecycle,
      graph: graphOf([
        { id: "hold", class: "control", kind: "wait", expires: { after: "48h0m0s" } },
      ]),
    });
    const wait = withWait.find(tile => tile.id === "wait");
    expect(wait?.body).toContain("gives up after 48h");
    expect(wait?.codes).toEqual(["wait", "expires"]);
    expect(
      buildLoopReliability({ effectiveLifecycle: null, graph: deliveryGraph }).map(t => t.id)
    ).not.toContain("wait");
  });

  it("Should always state the control verbs the runtime exposes, cancel distinct from kill", () => {
    const tiles = buildLoopReliability({ effectiveLifecycle: null, graph: graphOf([]) });
    const control = tiles.find(tile => tile.id === "control");
    expect(control?.body).toContain("Cancel lets in-flight work finish");
    expect(control?.body).toContain("closes the run as canceled");
    expect(control?.codes).toEqual(["cancel", "kill"]);
    expect(tiles.some(tile => tile.body.toLowerCase().includes("stop the run"))).toBe(false);
  });

  it("Should shorten Go durations and pass unparseable values through untouched", () => {
    expect(formatLoopDuration("2h0m0s")).toBe("2h");
    expect(formatLoopDuration("1h30m0s")).toBe("1h 30m");
    expect(formatLoopDuration("45m0s")).toBe("45m");
    expect(formatLoopDuration("30s")).toBe("30s");
    expect(formatLoopDuration("0s")).toBe("0s");
    expect(formatLoopDuration("soon")).toBe("soon");
    expect(formatLoopDuration(undefined)).toBe("");
  });
});
