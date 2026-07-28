import { describe, expect, it } from "vitest";

import { notificationKeys } from "../query-keys";

describe("notificationKeys", () => {
  it("scopes preset lists by every daemon query filter", () => {
    expect(
      notificationKeys.presetsList({
        enabled: true,
        built_in: false,
        name: " task_terminal ",
        limit: 25,
      })
    ).toEqual(["notifications", "presets", true, false, "task_terminal", 25]);
  });
});
