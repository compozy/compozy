import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { TokenListField } from "../token-list-field";

describe("TokenListField", () => {
  it("Should add, dedupe, and remove tokens", async () => {
    const onChange = vi.fn();
    const { rerender } = render(
      <TokenListField
        description="Tools"
        label="Tools"
        onChange={onChange}
        placeholder="Add tool"
        testId="agent-create-tools"
        values={["agh__skill_view"]}
      />
    );

    const user = userEvent.setup();
    const input = screen.getByTestId("agent-create-tools-input");
    await user.type(input, "mcp__github__*{enter}");
    expect(onChange).toHaveBeenCalledWith(["agh__skill_view", "mcp__github__*"]);

    onChange.mockClear();
    rerender(
      <TokenListField
        description="Tools"
        label="Tools"
        onChange={onChange}
        placeholder="Add tool"
        testId="agent-create-tools"
        values={["agh__skill_view", "mcp__github__*"]}
      />
    );
    await user.type(input, "agh__skill_view{enter}");
    expect(onChange).toHaveBeenCalledWith(["agh__skill_view", "mcp__github__*"]);

    onChange.mockClear();
    await user.click(screen.getByLabelText("Remove mcp__github__*"));
    expect(onChange).toHaveBeenCalledWith(["agh__skill_view"]);
  });
});
