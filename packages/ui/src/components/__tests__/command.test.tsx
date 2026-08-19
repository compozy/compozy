import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import { resolveCommandSelection } from "../../lib/command-selection";
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

interface ExternalItem {
  value: string;
  label: string;
}

/**
 * A palette driven the way a registry projection drives one: cmdk filters and
 * sorts nothing, the caller supplies both the visible set and its order, and
 * selection survives churn through `resolveCommandSelection`.
 */
function ExternalPalette({
  items,
  filterExternally = false,
  onSelect = () => {},
}: {
  items: readonly ExternalItem[];
  filterExternally?: boolean;
  onSelect?: (value: string) => void;
}) {
  const [query, setQuery] = useState("");
  const visible = filterExternally
    ? items.filter(item => item.label.toLowerCase().includes(query.trim().toLowerCase()))
    : items;
  const values = visible.map(item => item.value);
  const [selection, setSelection] = useState<{ previous: readonly string[]; value: string }>(
    () => ({ previous: values, value: values[0] ?? "" })
  );
  return (
    <Command
      shouldFilter={false}
      value={resolveCommandSelection(selection.previous, values, selection.value)}
      onValueChange={next => setSelection({ previous: values, value: next })}
    >
      <CommandInput
        aria-label="Search"
        placeholder="Search..."
        value={query}
        onValueChange={setQuery}
      />
      <CommandList>
        {/* `forceMount` keeps cmdk's match registry out of it: without it, cmdk
            recovers from an unmounting selected item by re-selecting the first
            one — the churn this contract has to survive. It also takes the rows
            out of cmdk's count, so the caller owns the empty state too. */}
        {visible.length === 0 ? <CommandEmpty>No results.</CommandEmpty> : null}
        {visible.map(item => (
          <CommandItem forceMount key={item.value} value={item.value} onSelect={onSelect}>
            {item.label}
          </CommandItem>
        ))}
      </CommandList>
    </Command>
  );
}

function itemLabels(container: HTMLElement): string[] {
  return Array.from(container.querySelectorAll("[data-slot='command-item']")).map(
    node => node.textContent?.trim() ?? ""
  );
}

function selectedValue(container: HTMLElement): string | null {
  return (
    container
      .querySelector("[data-slot='command-item'][data-selected='true']")
      ?.textContent?.trim() ?? null
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

  it("Should render externally-ordered items verbatim with shouldFilter disabled [UT-152]", async () => {
    const user = userEvent.setup();
    // Ranked by the caller, not alphabetically, and no label contains "zz" —
    // cmdk's own filter would drop every row and re-sort what survived.
    const items = [
      { value: "window.close", label: "Close window" },
      { value: "app.open.agents", label: "Open Agents" },
      { value: "session.new", label: "New session" },
    ];
    const { container } = render(<ExternalPalette items={items} />);
    expect(itemLabels(container)).toEqual(["Close window", "Open Agents", "New session"]);
    await user.type(screen.getByLabelText("Search"), "zz");
    await waitFor(() => expect(screen.getByLabelText("Search")).toHaveValue("zz"));
    expect(itemLabels(container)).toEqual(["Close window", "Open Agents", "New session"]);
    expect(screen.queryByText("No results.")).not.toBeInTheDocument();
  });

  it("Should select through the keyboard while the external item set is replaced [UT-153]", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    const first = [
      { value: "window.close", label: "Close window" },
      { value: "window.zoom", label: "Zoom window" },
      { value: "window.minimize", label: "Minimize window" },
    ];
    const { container, rerender } = render(
      <ExternalPalette filterExternally items={first} onSelect={onSelect} />
    );
    await user.click(screen.getByLabelText("Search"));
    await user.keyboard("{ArrowDown}");
    await waitFor(() => expect(selectedValue(container)).toBe("Zoom window"));
    // A live catalog wave lands between the arrow and the commit: the highlight
    // stays on the row the operator aimed at, and ⏎ fires that row.
    const replaced = [
      { value: "window.close", label: "Close window" },
      { value: "window.zoom", label: "Zoom window" },
      { value: "layout.balance", label: "Balance layout" },
    ];
    rerender(<ExternalPalette filterExternally items={replaced} onSelect={onSelect} />);
    await waitFor(() => expect(itemLabels(container)).toContain("Balance layout"));
    await user.keyboard("{Enter}");
    await waitFor(() => expect(onSelect).toHaveBeenCalledWith("window.zoom"));
  });

  it("Should keep the highlight across churn and fall to the nearest neighbour [UT-154]", async () => {
    const user = userEvent.setup();
    const items = [
      { value: "a", label: "Alpha" },
      { value: "b", label: "Bravo" },
      { value: "c", label: "Charlie" },
    ];
    const { container, rerender } = render(<ExternalPalette items={items} />);
    await user.click(screen.getByLabelText("Search"));
    await user.keyboard("{ArrowDown}");
    await waitFor(() => expect(selectedValue(container)).toBe("Bravo"));

    rerender(
      <ExternalPalette
        items={[
          { value: "c", label: "Charlie" },
          { value: "b", label: "Bravo" },
          { value: "a", label: "Alpha" },
        ]}
      />
    );
    await waitFor(() => expect(itemLabels(container)).toEqual(["Charlie", "Bravo", "Alpha"]));
    expect(selectedValue(container)).toBe("Bravo");

    rerender(
      <ExternalPalette
        items={[
          { value: "c", label: "Charlie" },
          { value: "a", label: "Alpha" },
        ]}
      />
    );
    await waitFor(() => expect(itemLabels(container)).toEqual(["Charlie", "Alpha"]));
    expect(selectedValue(container)).toBe("Alpha");
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

describe("resolveCommandSelection", () => {
  const rows = ["session:a", "session:b", "session:c"];

  it("Should keep the selection when its item survives the refresh", () => {
    expect(
      resolveCommandSelection(rows, ["session:c", "session:a", "session:b"], "session:b")
    ).toBe("session:b");
    expect(resolveCommandSelection(rows, [...rows, "session:d"], "session:a")).toBe("session:a");
  });

  it("Should move to the item that took its place when the selection disappears", () => {
    expect(resolveCommandSelection(rows, ["session:a", "session:c"], "session:b")).toBe(
      "session:c"
    );
    expect(resolveCommandSelection(rows, ["session:a", "session:b"], "session:c")).toBe(
      "session:b"
    );
    expect(resolveCommandSelection(rows, ["session:b"], "session:c")).toBe("session:b");
  });

  it("Should clear the selection only when nothing is left to select", () => {
    expect(resolveCommandSelection(rows, [], "session:b")).toBe("");
    expect(resolveCommandSelection([], rows, "")).toBe("session:a");
  });
});
