import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { SettingsChoiceGroup } from "../settings-choice-group";

const options = [
  { value: "ask", name: "Ask first", description: "Wait for approval." },
  { value: "read", name: "Allow reading", description: "Read without waiting." },
  { value: "all", name: "Allow everything", description: "Act without waiting." },
] as const;

describe("SettingsChoiceGroup", () => {
  it("Should move selection and focus with arrows, Home, and End", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <SettingsChoiceGroup
        ariaLabel="Permission mode"
        onChange={onChange}
        options={options}
        value="ask"
      />
    );

    const ask = screen.getByRole("radio", { name: /Ask first/u });
    ask.focus();
    await user.keyboard("{ArrowRight}");
    expect(onChange).toHaveBeenLastCalledWith("read");
    expect(screen.getByRole("radio", { name: /Allow reading/u })).toHaveFocus();

    await user.keyboard("{End}");
    expect(onChange).toHaveBeenLastCalledWith("all");
    expect(screen.getByRole("radio", { name: /Allow everything/u })).toHaveFocus();

    await user.keyboard("{Home}");
    expect(onChange).toHaveBeenLastCalledWith("ask");
    expect(ask).toHaveFocus();
  });

  it("Should reverse horizontal arrow movement in right-to-left layouts", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <div dir="rtl">
        <SettingsChoiceGroup
          ariaLabel="Permission mode"
          onChange={onChange}
          options={options}
          value="ask"
        />
      </div>
    );

    screen.getByRole("radio", { name: /Ask first/u }).focus();
    await user.keyboard("{ArrowRight}");

    expect(onChange).toHaveBeenLastCalledWith("all");
    expect(screen.getByRole("radio", { name: /Allow everything/u })).toHaveFocus();
  });
});
