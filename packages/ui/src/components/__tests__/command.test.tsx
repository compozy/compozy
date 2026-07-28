import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import {
  Command,
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from "../command";

function PaletteExample({ onSelect = () => {} }: { onSelect?: (value: string) => void }) {
  return (
    <Command>
      <CommandInput placeholder="Search..." aria-label="Search" />
      <CommandList>
        <CommandEmpty>No results.</CommandEmpty>
        <CommandGroup heading="Navigate">
          <CommandItem value="sessions" onSelect={onSelect}>
            Go to sessions
          </CommandItem>
          <CommandItem value="agents" onSelect={onSelect}>
            Go to agents
          </CommandItem>
        </CommandGroup>
        <CommandSeparator />
        <CommandGroup heading="Actions">
          <CommandItem value="new" onSelect={onSelect}>
            Start new session
          </CommandItem>
        </CommandGroup>
      </CommandList>
    </Command>
  );
}

describe("Command", () => {
  it("Should hide the decorative search icon from assistive technologies", () => {
    const { container } = render(<PaletteExample />);
    const searchIcon = container.querySelector("[data-slot='command-input-group'] svg");
    expect(searchIcon).toHaveAttribute("aria-hidden", "true");
  });

  it("Should filter items as the user types", async () => {
    const user = userEvent.setup();
    render(<PaletteExample />);
    expect(screen.getByText("Go to sessions")).toBeInTheDocument();
    expect(screen.getByText("Start new session")).toBeInTheDocument();
    await user.type(screen.getByLabelText("Search"), "agents");
    await waitFor(() => expect(screen.queryByText("Start new session")).not.toBeInTheDocument());
    expect(screen.getByText("Go to agents")).toBeInTheDocument();
  });

  it("Should render the empty state when no items match", async () => {
    const user = userEvent.setup();
    render(<PaletteExample />);
    await user.type(screen.getByLabelText("Search"), "zzzz");
    await waitFor(() => expect(screen.getByText("No results.")).toBeInTheDocument());
  });

  it("Should select the highlighted item on Enter and fire onSelect", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(<PaletteExample onSelect={onSelect} />);
    await user.click(screen.getByLabelText("Search"));
    await user.keyboard("{ArrowDown}");
    await user.keyboard("{Enter}");
    await waitFor(() => expect(onSelect).toHaveBeenCalled());
    expect(onSelect.mock.calls.at(-1)?.[0]).toBeTypeOf("string");
  });

  it("Should render inside CommandDialog when open", async () => {
    render(
      <CommandDialog open>
        <Command>
          <CommandInput placeholder="Search..." aria-label="Search" />
          <CommandList>
            <CommandItem value="a">Option A</CommandItem>
          </CommandList>
        </Command>
      </CommandDialog>
    );
    await waitFor(() => expect(screen.getByRole("dialog")).toBeInTheDocument());
    expect(within(document.body).getByText("Option A")).toBeInTheDocument();
  });
});
