import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectSeparator,
  SelectTrigger,
  SelectValue,
} from "../select";

function SelectExample() {
  return (
    <Select>
      <SelectTrigger aria-label="Agent">
        <SelectValue placeholder="Pick an agent" />
      </SelectTrigger>
      <SelectContent>
        <SelectGroup>
          <SelectLabel>Local</SelectLabel>
          <SelectItem value="claude">Claude Code</SelectItem>
          <SelectItem value="codex">Codex CLI</SelectItem>
        </SelectGroup>
        <SelectSeparator />
        <SelectGroup>
          <SelectLabel>Remote</SelectLabel>
          <SelectItem value="gemini">Gemini CLI</SelectItem>
        </SelectGroup>
      </SelectContent>
    </Select>
  );
}

describe("Select", () => {
  it("Should render the placeholder before the trigger is activated", () => {
    render(<SelectExample />);
    expect(screen.getByText("Pick an agent")).toBeInTheDocument();
  });

  it("Should open a listbox with grouped options on click", async () => {
    const user = userEvent.setup();
    render(<SelectExample />);
    await user.click(screen.getByRole("combobox", { name: "Agent" }));
    await waitFor(() => expect(screen.getByRole("listbox")).toBeInTheDocument());
    expect(screen.getByText("Claude Code")).toBeInTheDocument();
    expect(screen.getByText("Codex CLI")).toBeInTheDocument();
    expect(screen.getByText("Gemini CLI")).toBeInTheDocument();
    const groupLabels = screen.getAllByText(/Local|Remote/);
    expect(groupLabels.length).toBeGreaterThanOrEqual(2);
  });

  it("Should close on Escape after opening", async () => {
    const user = userEvent.setup();
    render(<SelectExample />);
    await user.click(screen.getByRole("combobox", { name: "Agent" }));
    await waitFor(() => expect(screen.getByRole("listbox")).toBeInTheDocument());
    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("listbox")).not.toBeInTheDocument(), {
      timeout: 1500,
    });
  });

  it("Should apply data-size to the trigger", () => {
    const { container } = render(
      <Select>
        <SelectTrigger size="sm">
          <SelectValue placeholder="Pick" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="a">A</SelectItem>
        </SelectContent>
      </Select>
    );
    const trigger = container.querySelector("[data-slot=select-trigger]") as HTMLElement | null;
    expect(trigger).toHaveAttribute("data-size", "sm");
  });
});
