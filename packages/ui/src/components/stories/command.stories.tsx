import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, waitFor, within } from "storybook/test";
import { useState } from "react";
import {
  CircleDotIcon,
  CpuIcon,
  DatabaseIcon,
  LayersIcon,
  SearchIcon,
  SettingsIcon,
} from "lucide-react";

import { resolveCommandSelection } from "../../lib/command-selection";
import { Kbd, KbdGroup } from "../kbd";
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
  CommandShortcut,
} from "../command";

const meta: Meta<typeof Command> = {
  title: "components/ui/Command",
  component: Command,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "cmdk-powered palette. Use as a standalone panel or inside a CommandDialog. Arrow keys move between items; Enter selects.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const navigateItems = [
  { value: "sessions", label: "Go to sessions", icon: CircleDotIcon },
  { value: "agents", label: "Go to agents", icon: CpuIcon },
  { value: "skills", label: "Go to skills", icon: LayersIcon },
  { value: "memory", label: "Go to memory", icon: DatabaseIcon },
];

const quickItems = [
  { value: "new-session", label: "Start new session", shortcut: "⌘N" },
  { value: "search", label: "Search events", shortcut: "⌘F" },
  { value: "settings", label: "Open settings", icon: SettingsIcon },
];

export const Default: Story = {
  render: () => (
    <Command className="w-[24rem] border">
      <CommandInput placeholder="Type a command or search..." />
      <CommandList>
        <CommandEmpty>No results found.</CommandEmpty>
        <CommandGroup heading="Navigate">
          {navigateItems.map(item => (
            <CommandItem key={item.value} value={item.value}>
              <item.icon />
              {item.label}
            </CommandItem>
          ))}
        </CommandGroup>
        <CommandSeparator />
        <CommandGroup heading="Actions">
          {quickItems.map(item => (
            <CommandItem key={item.value} value={item.value}>
              {item.icon ? <item.icon /> : <SearchIcon />}
              {item.label}
              {item.shortcut ? <CommandShortcut>{item.shortcut}</CommandShortcut> : null}
            </CommandItem>
          ))}
        </CommandGroup>
      </CommandList>
    </Command>
  ),
};

export const WithShortcuts: Story = {
  render: () => (
    <Command className="w-[24rem] border">
      <CommandInput placeholder="Jump to..." />
      <CommandList>
        <CommandEmpty>No results found.</CommandEmpty>
        <CommandGroup heading="Suggestions">
          <CommandItem value="palette">
            Open command palette
            <KbdGroup>
              <Kbd>⌘</Kbd>
              <Kbd>K</Kbd>
            </KbdGroup>
          </CommandItem>
          <CommandItem value="toggle-sidebar">
            Toggle sidebar
            <KbdGroup>
              <Kbd>⌘</Kbd>
              <Kbd>B</Kbd>
            </KbdGroup>
          </CommandItem>
        </CommandGroup>
      </CommandList>
    </Command>
  ),
};

export const KeyboardNavigation: Story = {
  parameters: {
    docs: {
      description: {
        story:
          "Filter by typing, then Arrow keys highlight items and Enter selects the highlighted one.",
      },
    },
  },
  render: () => (
    <Command className="w-[24rem] border">
      <CommandInput placeholder="Search..." aria-label="Search palette" />
      <CommandList>
        <CommandEmpty>No results found.</CommandEmpty>
        <CommandGroup heading="Navigate">
          {navigateItems.map(item => (
            <CommandItem key={item.value} value={item.value}>
              <item.icon />
              {item.label}
            </CommandItem>
          ))}
        </CommandGroup>
      </CommandList>
    </Command>
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const search = await canvas.findByRole("combobox", { name: "Search palette" });
    await userEvent.click(search);
    await userEvent.type(search, "agents");
    await waitFor(() => expect(canvas.getByText("Go to agents")).toBeInTheDocument());
    await expect(canvas.queryByText("Go to memory")).not.toBeInTheDocument();
  },
};

const rankedItems = [
  { value: "window.close", label: "Close window" },
  { value: "app.open.agents", label: "Open Agents" },
  { value: "session.new", label: "New session" },
  { value: "layout.balance", label: "Balance layout" },
];

export const ExternalOrdering: Story = {
  parameters: {
    docs: {
      description: {
        story:
          "`shouldFilter={false}` hands ordering and membership to the caller: rows render in the supplied order and typing never re-sorts or drops them.",
      },
    },
  },
  render: () => (
    <Command className="w-[24rem] border" shouldFilter={false}>
      <CommandInput aria-label="Search palette" placeholder="Ranked by the caller..." />
      <CommandList>
        <CommandEmpty>No results found.</CommandEmpty>
        <CommandGroup heading="Ranked">
          {rankedItems.map(item => (
            <CommandItem key={item.value} value={item.value}>
              {item.label}
            </CommandItem>
          ))}
        </CommandGroup>
      </CommandList>
    </Command>
  ),
};

function ChurningPalette() {
  const [items, setItems] = useState(rankedItems);
  const values = items.map(item => item.value);
  const [selection, setSelection] = useState<{ previous: readonly string[]; value: string }>(
    () => ({ previous: values, value: values[0] ?? "" })
  );
  return (
    <div className="flex w-[24rem] flex-col gap-2">
      <Command
        className="border"
        shouldFilter={false}
        value={resolveCommandSelection(selection.previous, values, selection.value)}
        onValueChange={next => setSelection({ previous: values, value: next })}
      >
        <CommandInput aria-label="Search palette" placeholder="Arrow down, then churn..." />
        <CommandList>
          <CommandEmpty>No results found.</CommandEmpty>
          <CommandGroup heading="Live catalog">
            {items.map(item => (
              <CommandItem key={item.value} value={item.value}>
                {item.label}
              </CommandItem>
            ))}
          </CommandGroup>
        </CommandList>
      </Command>
      <button
        className="self-start rounded-md border border-line px-2 py-1 text-small-body text-fg"
        type="button"
        onClick={() => setItems(current => current.filter(item => item.value !== "session.new"))}
      >
        Drop “New session”
      </button>
    </div>
  );
}

export const SelectionSurvival: Story = {
  tags: ["play-fn"],
  parameters: {
    docs: {
      description: {
        story:
          "A live list that churns underneath the operator. `resolveCommandSelection` keeps the highlight on the surviving row, and falls to the nearest neighbour only when the selected row is gone.",
      },
    },
  },
  render: () => <ChurningPalette />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const search = await canvas.findByRole("combobox", { name: "Search palette" });
    await userEvent.click(search);
    await userEvent.keyboard("{ArrowDown}{ArrowDown}");
    const selected = () =>
      canvasElement.querySelector("[data-slot='command-item'][data-selected='true']");
    await waitFor(() => expect(selected()).toHaveTextContent("New session"));
    await userEvent.click(canvas.getByRole("button", { name: /Drop/ }));
    await waitFor(() => expect(canvas.queryByText("New session")).not.toBeInTheDocument());
    await expect(selected()).toHaveTextContent("Balance layout");
  },
};
