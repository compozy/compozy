import { describe, expect, it } from "vitest";

import {
  bindingKindLabel,
  bindingsGist,
  countLoopBindings,
  summarizeBindingKinds,
  type LoopBindingRow,
} from "../loop-bindings";

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

  it("Should count rendered rows when no paginated source is supplied", () => {
    expect(countLoopBindings(3)).toEqual({ paginated: false, hasMore: false, loaded: 3, total: 3 });
  });

  it("Should sum paginated schedule and trigger counts", () => {
    expect(
      countLoopBindings(
        5,
        { hasMore: true, loaded: 2, total: 4 },
        { hasMore: false, loaded: 3, total: 3 }
      )
    ).toEqual({ paginated: true, hasMore: true, loaded: 5, total: 7 });
    expect(countLoopBindings(1, undefined, { hasMore: false, loaded: 1, total: 1 })).toEqual({
      paginated: true,
      hasMore: false,
      loaded: 1,
      total: 1,
    });
  });

  it("Should describe the attached automation count", () => {
    expect(bindingsGist(0)).toBe("Manual only");
    expect(bindingsGist(1)).toBe("1 automation");
    expect(bindingsGist(4)).toBe("4 automations");
  });
});
