// Suite: palette argument dropdown
// Invariant: Enter picks or submits once, and option ids stay unique for duplicate labels.
// Boundary IN: PaletteArgDropdown keyboard handling and option identity.
// Boundary OUT: PaletteArgsBar submit and cmd-palette-args coerce.
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { PaletteArgField } from "../../lib/cmd-palette-args";
import { PaletteArgDropdown } from "../os-palette-arg-dropdown";

function field(overrides: Partial<PaletteArgField> = {}): PaletteArgField {
  return {
    name: "mode",
    type: "dropdown",
    required: true,
    placeholder: "Pick a mode",
    options: ["fast", "slow", "fast"],
    value: "",
    error: "",
    ...overrides,
  };
}

describe("palette argument dropdown", () => {
  it("Should consume Enter and submit only once when the list is closed [RD0041]", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn();
    render(
      <PaletteArgDropdown
        className=""
        field={field({ options: [] })}
        focused
        registerNode={() => {}}
        onChange={vi.fn()}
        onSubmit={onSubmit}
      />
    );

    await user.click(screen.getByRole("combobox"));
    await user.keyboard("{Enter}");
    expect(onSubmit).toHaveBeenCalledTimes(1);
  });

  it("Should pick the highlighted option on Enter without submitting [RA0174]", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    const onSubmit = vi.fn();
    render(
      <PaletteArgDropdown
        className=""
        field={field({ options: ["fast", "slow"] })}
        focused
        registerNode={() => {}}
        onChange={onChange}
        onSubmit={onSubmit}
      />
    );

    await user.click(screen.getByRole("combobox"));
    await user.keyboard("{ArrowDown}{Enter}");
    expect(onChange).toHaveBeenCalledWith("slow");
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("Should give duplicate labels distinct option ids [RA0175]", async () => {
    const user = userEvent.setup();
    render(
      <PaletteArgDropdown
        className=""
        field={field({ value: "f" })}
        focused
        registerNode={() => {}}
        onChange={vi.fn()}
        onSubmit={vi.fn()}
      />
    );

    await user.click(screen.getByRole("combobox"));
    const options = screen.getAllByRole("option");
    expect(options).toHaveLength(2);
    expect(options[0]?.id).toBe("os-palette-arg-options-mode-option-0");
    expect(options[1]?.id).toBe("os-palette-arg-options-mode-option-1");
    expect(options[0]?.id).not.toBe(options[1]?.id);
  });
});
