import { describe, expect, it } from "vitest";

import {
  setLoopTargetInput,
  setLoopTargetLoop,
  setLoopTargetMapping,
  type LoopTargetDraft,
} from "../loop-target";

const base: LoopTargetDraft = {
  loop_name: "software-delivery",
  inputs: { slug: "x" },
  input_mapping: {},
};

describe("loop-target", () => {
  it("Should switch the target loop and reset inputs + mapping tied to the old schema", () => {
    const next = setLoopTargetLoop(base, "reviews-watch");
    expect(next.loop_name).toBe("reviews-watch");
    expect(next.inputs).toEqual({});
    expect(next.input_mapping).toEqual({});
  });

  it("Should keep the draft identity when the loop is unchanged", () => {
    expect(setLoopTargetLoop(base, "software-delivery")).toBe(base);
  });

  it("Should set and clear static input values", () => {
    const set = setLoopTargetInput(base, "branch", "main");
    expect(set.inputs).toEqual({ slug: "x", branch: "main" });
    const cleared = setLoopTargetInput(set, "branch", "");
    expect(cleared.inputs).toEqual({ slug: "x" });
    expect(setLoopTargetInput(set, "slug", undefined).inputs).toEqual({ branch: "main" });
    expect(setLoopTargetInput(set, "slug", null).inputs).toEqual({ branch: "main" });
  });

  it("Should set and clear event-payload mapping paths", () => {
    const mapped = setLoopTargetMapping(base, "slug", "trigger.payload.slug");
    expect(mapped.input_mapping).toEqual({ slug: "trigger.payload.slug" });
    const cleared = setLoopTargetMapping(mapped, "slug", "   ");
    expect(cleared.input_mapping).toEqual({});
  });
});
