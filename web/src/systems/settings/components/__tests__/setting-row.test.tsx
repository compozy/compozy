import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { SettingRow } from "../setting-row";

describe("SettingRow", () => {
  it("associates fragment controls with the row label and description", () => {
    render(
      <SettingRow
        control={
          <>
            <input aria-label="value" />
            <span>seconds</span>
          </>
        }
        description="How long the daemon waits"
        label="Timeout"
      />
    );

    const controlGroup = screen.getByRole("group", { name: "Timeout" });
    expect(controlGroup).toHaveAccessibleDescription("How long the daemon waits");
  });
});
