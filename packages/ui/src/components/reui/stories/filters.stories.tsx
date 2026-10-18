import type { Meta, StoryObj } from "@storybook/react-vite";
import { AtSign, Bot, Briefcase, CircleDot, ListFilter, Pin } from "lucide-react";
import { useState } from "react";

import { Button } from "../../button";
import { Filters, FiltersWithSearch, type Filter, type FilterFieldsConfig } from "../filters";

const TOGGLE_FIELDS: FilterFieldsConfig<boolean> = [
  {
    key: "has_work",
    label: "Has work",
    icon: <Briefcase aria-hidden="true" className="size-3" />,
    type: "toggle",
  },
  {
    key: "mentions_me",
    label: "@me",
    icon: <AtSign aria-hidden="true" className="size-3" />,
    type: "toggle",
  },
  {
    key: "pinned",
    label: "Pinned",
    icon: <Pin aria-hidden="true" className="size-3" />,
    type: "toggle",
  },
  {
    key: "unread",
    label: "Unread",
    icon: <CircleDot aria-hidden="true" className="size-3" />,
    type: "toggle",
  },
];

const TRIGGER = (
  <Button size="sm" variant="quiet" aria-label="Add filter">
    <ListFilter aria-hidden="true" className="size-3" />
    Filter
  </Button>
);

function ToggleFiltersDemo({ initial = [] as Filter<boolean>[] }) {
  const [filters, setFilters] = useState<Filter<boolean>[]>(initial);

  return (
    <div className="w-full max-w-2xl rounded-md border border-line p-3">
      <Filters<boolean>
        allowMultiple={false}
        fields={TOGGLE_FIELDS}
        filters={filters}
        onChange={setFilters}
        size="sm"
        trigger={TRIGGER}
      />
    </div>
  );
}

const meta: Meta<typeof ToggleFiltersDemo> = {
  title: "components/reui/Filters",
  component: ToggleFiltersDemo,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Filters with `type: 'toggle'` fields render as one-click boolean chips: no operator dropdown, no value selector. Picking from the +Filter menu adds the chip immediately; the trailing X removes it.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Empty: Story = {
  name: "Toggle — empty",
  render: () => <ToggleFiltersDemo />,
};

export const WithTwoChips: Story = {
  name: "Toggle — two chips",
  render: () => (
    <ToggleFiltersDemo
      initial={[
        { id: "chip-has-work", field: "has_work", operator: "is", values: [true] },
        { id: "chip-unread", field: "unread", operator: "is", values: [true] },
      ]}
    />
  ),
};

export const AllChipsActive: Story = {
  name: "Toggle — all four chips",
  render: () => (
    <ToggleFiltersDemo
      initial={[
        { id: "chip-1", field: "has_work", operator: "is", values: [true] },
        { id: "chip-2", field: "mentions_me", operator: "is", values: [true] },
        { id: "chip-3", field: "pinned", operator: "is", values: [true] },
        { id: "chip-4", field: "unread", operator: "is", values: [true] },
      ]}
    />
  ),
};

function SearchableFiltersDemo() {
  const [filters, setFilters] = useState<Filter<boolean>[]>([]);

  return (
    <div className="w-full max-w-2xl rounded-md border border-line p-3">
      <FiltersWithSearch<boolean>
        allowMultiple={false}
        fields={TOGGLE_FIELDS}
        filters={filters}
        onChange={setFilters}
        size="sm"
        trigger={TRIGGER}
      />
    </div>
  );
}

export const WithMenuSearch: Story = {
  name: "With menu search",
  render: () => <SearchableFiltersDemo />,
};

const AGENT_FIELDS: FilterFieldsConfig<string> = [
  {
    key: "agent",
    label: "Agent",
    icon: <Bot aria-hidden="true" />,
    type: "select",
    options: [
      { value: "codex", label: "codex" },
      { value: "claude", label: "claude" },
    ],
  },
];

function DefaultSizeFiltersDemo() {
  const [filters, setFilters] = useState<Filter<string>[]>([
    { id: "chip-agent", field: "agent", operator: "is", values: ["codex"] },
  ]);

  return (
    <div className="w-full max-w-2xl rounded-md border border-line p-3">
      <Filters<string>
        fields={AGENT_FIELDS}
        filters={filters}
        onChange={setFilters}
        trigger={
          <Button variant="quiet" aria-label="Add filter">
            <ListFilter aria-hidden="true" />
            Filter
          </Button>
        }
      />
    </div>
  );
}

/** Default size: a select chip (label · operator · value · remove) as one 30 px pill. */
export const DefaultSizeSelectChip: Story = {
  name: "Select chip — default size",
  render: () => <DefaultSizeFiltersDemo />,
};
