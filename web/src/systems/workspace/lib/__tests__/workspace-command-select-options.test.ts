import { describe, expect, it } from "vitest";

import { toWorkspaceCommandSelectOptions } from "../workspace-command-select-options";

describe("toWorkspaceCommandSelectOptions", () => {
  it("Should preserve root_dir so home workspace detection can match user_home_dir", () => {
    const options = toWorkspaceCommandSelectOptions([
      {
        id: "ws_home",
        name: "pedronauck",
        root_dir: "/Users/pedronauck",
      },
      {
        id: "ws_other",
        name: "mastra-video",
        root_dir: "/Users/pedronauck/projects/mastra-video",
      },
    ]);

    expect(options).toEqual([
      { id: "ws_home", name: "pedronauck", root_dir: "/Users/pedronauck" },
      { id: "ws_other", name: "mastra-video", root_dir: "/Users/pedronauck/projects/mastra-video" },
    ]);
  });
});
