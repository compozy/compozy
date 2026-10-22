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

  it("Should keep the window's data binding so an item window opened in place stays bound", () => {
    expect(validateTaskDetailSearch({ tab: "runs", workspace: " ws_beta " })).toEqual({
      tab: "runs",
      workspace: "ws_beta",
    });
    expect(validateTaskDetailSearch({ workspace: "@user" })).toEqual({
      tab: "overview",
      workspace: "@user",
    });
    expect(validateTaskDetailSearch({ workspace: "  " })).toEqual({ tab: "overview" });
  });

  it("Should reject invalid URL enums without retaining untrusted values", () => {
    expect(validateTaskDetailSearch({ tab: "orchestration", inspect: "secrets" })).toEqual({
      tab: "overview",
    });
  });
});
