import { describe, expect, it } from "vitest";

import { bindingKindLabel, summarizeBindingKinds, type LoopBindingRow } from "../loop-bindings";

const rows: LoopBindingRow[] = [
  { id: "1", name: "nightly", kind: "schedule", enabled: true, meta: "" },
  { id: "2", name: "webhook", kind: "webhook", enabled: false, meta: "" },
  { id: "3", name: "another-schedule", kind: "schedule", enabled: true, meta: "" },
];

describe("loop-bindings", () => {
  it("Should label each binding kind", () => {
    expect(bindingKindLabel("schedule")).toBe("schedule");
    expect(bindingKindLabel("webhook")).toBe("webhook");
    expect(bindingKindLabel("trigger")).toBe("trigger");
  });

  it("Should summarize the distinct binding kinds, sorted and deduplicated", () => {
    expect(summarizeBindingKinds(rows)).toEqual(["schedule", "webhook"]);
    expect(summarizeBindingKinds([])).toEqual([]);
  });
});
