import { describe, expect, it, vi } from "vitest";

import { LOOP_RUN_STATUSES } from "@/generated/loop-enums";

import {
  applyLoopFilterChips,
  buildLoopFilterFields,
  loopFiltersToChips,
  loopStatusFilterOptions,
  parseLoopCategoryFilter,
  parseLoopKindFilter,
  parseLoopStatusFilter,
  type LoopFilterHandlers,
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

describe("buildLoopFilterFields", () => {
  it("Should expose one status select carrying the full daemon vocabulary", () => {
    const fields = buildLoopFilterFields();
    const [field] = fields;
    if (!field || !("key" in field)) throw new Error("loop filters must be a flat select field");

    expect(fields).toHaveLength(1);
    expect(field.key).toBe("status");
    expect(field.type).toBe("select");
    expect(field.options?.map(option => option.value)).toEqual([...LOOP_RUN_STATUSES]);
    expect(field.options?.find(option => option.value === "canceled")?.label).toBe("Canceled");
  });
});

describe("loopFiltersToChips", () => {
  it("Should project only a concrete status filter", () => {
    expect(loopFiltersToChips({ status: "canceled" })).toEqual([
      { field: "status", id: "loop-filter-status", operator: "is", values: ["canceled"] },
    ]);
    expect(loopFiltersToChips({ status: null })).toEqual([]);
  });
});

describe("applyLoopFilterChips", () => {
  function createHandlers(): LoopFilterHandlers {
    return { onStatusChange: vi.fn() };
  }

  it("Should dispatch the selected status to its typed handler", () => {
    const handlers = createHandlers();

    applyLoopFilterChips(
      [{ field: "status", id: "loop-filter-status", operator: "is", values: ["canceled"] }],
      handlers
    );

    expect(handlers.onStatusChange).toHaveBeenCalledWith("canceled");
  });

  it("Should clear the status when the chip is removed or carries an unknown value", () => {
    const handlers = createHandlers();

    applyLoopFilterChips([], handlers);
    expect(handlers.onStatusChange).toHaveBeenLastCalledWith(null);

    applyLoopFilterChips(
      [{ field: "status", id: "loop-filter-status", operator: "is", values: ["stop"] }],
      handlers
    );
    expect(handlers.onStatusChange).toHaveBeenLastCalledWith(null);
  });
});
