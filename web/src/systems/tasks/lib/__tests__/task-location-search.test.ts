import { describe, expect, it } from "vitest";

import { taskModeSearchFor } from "../task-location-search";

describe("taskModeSearchFor", () => {
  it("Should clear the list query while preserving the remaining catalog state", () => {
    expect(
      taskModeSearchFor("kanban", {
        query: "review",
        status: "ready",
        owner: "agent_session:codex",
        sort: "priority",
      })
    ).toEqual({
      mode: "kanban",
      status: "ready",
      owner: "agent_session:codex",
      sort: "priority",
    });
  });
});
