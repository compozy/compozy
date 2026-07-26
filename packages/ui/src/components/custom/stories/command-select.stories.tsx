import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";

import { CommandEmpty, CommandItem, CommandList } from "../../command";
import {
  CommandSelect,
  CommandSelectChip,
  CommandSelectChipStrip,
  CommandSelectGroup,
  CommandSelectShell,
  CommandSelectTrigger,
} from "../command-select";

const meta: Meta<typeof CommandSelect> = {
  title: "components/custom/CommandSelect",
  component: CommandSelect,
  parameters: {
    layout: "centered",
  },
  decorators: [
    Story => (
      <div className="w-[360px]">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

const models = ["gpt-5.4", "gpt-5.4-mini", "claude-opus", "claude-sonnet"];

function SingleHarness() {
  const [value, setValue] = useState("");
  const [open, setOpen] = useState(false);
  return (
    <CommandSelect open={open} onOpenChange={setOpen}>
      <CommandSelectTrigger
        label={value || "Select model"}
        selected={value !== ""}
        aria-expanded={open}
      />
      <CommandSelectShell inputPlaceholder="Filter models">
        <CommandList>
          <CommandEmpty>No models</CommandEmpty>
          <CommandSelectGroup heading="Models">
            {models.map(model => (
              <CommandItem
                key={model}
                value={model}
                data-checked={value === model ? "true" : "false"}
                onSelect={() => {
                  setValue(model);
                  setOpen(false);
                }}
              >
                {model}
              </CommandItem>
            ))}
          </CommandSelectGroup>
        </CommandList>
      </CommandSelectShell>
    </CommandSelect>
  );
}

function MultiHarness() {
  const [selected, setSelected] = useState(["gpt-5.4", "claude-opus"]);
  return (
    <CommandSelect>
      <CommandSelectTrigger>
        <CommandSelectChipStrip>
          {selected.map(model => (
            <CommandSelectChip
              key={model}
              onRemove={() => setSelected(current => current.filter(item => item !== model))}
            >
              {model}
            </CommandSelectChip>
          ))}
        </CommandSelectChipStrip>
      </CommandSelectTrigger>
    </CommandSelect>
  );
}

export const SingleSelect: Story = {
  args: {},
  render: () => <SingleHarness />,
};

export const MultiSelectChips: Story = {
  args: {},
  render: () => <MultiHarness />,
};
