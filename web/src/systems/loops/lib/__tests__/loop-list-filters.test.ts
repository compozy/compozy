import { describe, expect, it } from "vitest";

import { LOOP_RUN_STATUSES } from "@/generated/loop-enums";

import {
  loopStatusFilterOptions,
  parseLoopCategoryFilter,
  parseLoopKindFilter,
  parseLoopStatusFilter,
} from "../loop-list-filters";

describe("loop-list-filters", () => {
  it("Should offer every daemon status, including canceled, regardless of the loaded page", () => {
    const options = loopStatusFilterOptions();
    expect(options.map(option => option.value)).toEqual([...LOOP_RUN_STATUSES]);
    expect(options.map(option => option.value)).toContain("canceled");
    expect(options.find(option => option.value === "canceled")?.label).toBe("Canceled");
    expect(options.find(option => option.value === "needs-approval")?.label).toBe("Needs Approval");
  });

  it("Should parse URL search values and reject unknowns", () => {
    expect(parseLoopKindFilter("workspace")).toBe("workspace");
    expect(parseLoopKindFilter("all")).toBeUndefined();
    expect(parseLoopKindFilter("builtin")).toBeUndefined();
    expect(parseLoopCategoryFilter(" delivery ")).toBe("delivery");
    expect(parseLoopCategoryFilter("")).toBeUndefined();
    expect(parseLoopStatusFilter("running")).toBe("running");
    expect(parseLoopStatusFilter("canceled")).toBe("canceled");
    expect(parseLoopStatusFilter("stop")).toBeUndefined();
    expect(parseLoopStatusFilter("mystery")).toBeUndefined();
  });
});
