import { describe, expect, it } from "vitest";

import { parseTaskWindowLocation } from "../task-window-location";

describe("parseTaskWindowLocation", () => {
  it("Should fall back to the list mode when the tasks search mode is invalid", () => {
    expect(parseTaskWindowLocation({ pathname: "/tasks", search: { mode: "bogus" } })).toEqual({
      kind: "catalog",
      mode: "list",
    });
  });

  it("Should resolve task detail tab and inspect defaults at the window boundary", () => {
    expect(
      parseTaskWindowLocation({
        pathname: "/tasks/task_1",
        search: { inspect: "stream", tab: "activity" },
      })
    ).toEqual({
      kind: "detail",
      taskId: "task_1",
      search: { inspect: "stream", tab: "activity" },
    });
  });

  it("Should carry the catalog view the create dialog is layered over", () => {
    expect(
      parseTaskWindowLocation({
        pathname: "/tasks/new",
        search: { mode: "kanban", template: "recurring" },
      })
    ).toEqual({
      kind: "create",
      mode: "kanban",
      search: { mode: "kanban", template: "recurring" },
    });
  });

  it("Should default the create dialog background to the list view", () => {
    expect(parseTaskWindowLocation({ pathname: "/tasks/new", search: {} })).toEqual({
      kind: "create",
      mode: "list",
      search: {},
    });
  });

  it("Should carry the detail tab the edit dialog is layered over", () => {
    expect(
      parseTaskWindowLocation({ pathname: "/tasks/task_1/edit", search: { tab: "runs" } })
    ).toEqual({
      kind: "edit",
      taskId: "task_1",
      search: { tab: "runs" },
    });
  });
});
