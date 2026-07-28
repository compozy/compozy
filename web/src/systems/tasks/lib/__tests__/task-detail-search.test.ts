import { describe, expect, it } from "vitest";

import { validateTaskDetailSearch } from "../task-detail-search";

describe("validateTaskDetailSearch", () => {
  it("Should resolve an empty search to the overview tab exactly once", () => {
    expect(validateTaskDetailSearch({})).toEqual({ tab: "overview" });
  });

  it("Should preserve a valid tab and inspect deep link", () => {
    expect(validateTaskDetailSearch({ tab: "runs", inspect: "stream" })).toEqual({
      tab: "runs",
      inspect: "stream",
    });
  });

  it("Should reject invalid URL enums without retaining untrusted values", () => {
    expect(validateTaskDetailSearch({ tab: "orchestration", inspect: "secrets" })).toEqual({
      tab: "overview",
    });
  });
});
