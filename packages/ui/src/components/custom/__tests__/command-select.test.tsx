import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { CommandEmpty, CommandItem, CommandList } from "../../command";
import {
  CommandSelect,
  CommandSelectChip,
  CommandSelectChipStrip,
  CommandSelectGroup,
  CommandSelectShell,
  CommandSelectTrigger,
} from "../command-select";

describe("CommandSelect", () => {
  it("Should render the canonical trigger shell", () => {
    render(
      <CommandSelect>
        <CommandSelectTrigger label="claude-sonnet" selected data-testid="command-select-trigger" />
      </CommandSelect>
    );

    const trigger = screen.getByTestId("command-select-trigger");
    expect(trigger).toHaveAttribute("data-slot", "command-select-trigger");
    expect(trigger).toHaveTextContent("claude-sonnet");
  });

  it("Should render shell, group, and command items when opened", () => {
    render(
      <CommandSelect open>
        <CommandSelectTrigger label="Select model" />
        <CommandSelectShell inputPlaceholder="Filter models">
          <CommandList>
            <CommandEmpty>No models</CommandEmpty>
            <CommandSelectGroup heading="Models" data-testid="command-select-group">
              <CommandItem value="opus">Opus</CommandItem>
            </CommandSelectGroup>
          </CommandList>
        </CommandSelectShell>
      </CommandSelect>
    );

    expect(screen.getByTestId("command-select-group")).toHaveAttribute(
      "data-slot",
      "command-select-group"
    );
    expect(screen.getByPlaceholderText("Filter models")).toBeInTheDocument();
    expect(screen.getByText("Opus")).toBeInTheDocument();
  });

  it("Should render removable chips for multi-select consumers", () => {
    const onRemove = vi.fn();
    render(
      <CommandSelectChipStrip>
        <CommandSelectChip onRemove={onRemove}>codex</CommandSelectChip>
      </CommandSelectChipStrip>
    );

    const chip = screen.getByRole("button", { name: /codex/i });
    expect(chip).toHaveAttribute("data-slot", "command-select-chip");
    fireEvent.click(chip);
    expect(onRemove).toHaveBeenCalledTimes(1);
  });
});
