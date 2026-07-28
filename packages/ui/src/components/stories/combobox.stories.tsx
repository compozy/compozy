import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, waitFor, within } from "storybook/test";

import {
  Combobox,
  ComboboxChip,
  ComboboxChips,
  ComboboxChipsInput,
  ComboboxCollection,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
  useComboboxAnchor,
} from "../combobox";

const meta: Meta<typeof Combobox> = {
  title: "components/ui/Combobox",
  component: Combobox,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Base UI Combobox with autofilter. Compose `<ComboboxInput>` for single-select or `<ComboboxChips>` + `<ComboboxChipsInput>` for multi-select.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

type CityOption = { value: string; label: string };

const cities: CityOption[] = [
  { value: "albuquerque", label: "Albuquerque" },
  { value: "alexandria", label: "Alexandria" },
  { value: "amsterdam", label: "Amsterdam" },
  { value: "berlin", label: "Berlin" },
  { value: "boston", label: "Boston" },
  { value: "chicago", label: "Chicago" },
  { value: "seattle", label: "Seattle" },
];

export const Default: Story = {
  render: () => (
    <div className="w-80">
      <Combobox items={cities}>
        <ComboboxInput placeholder="Search cities" />
        <ComboboxContent>
          <ComboboxList>
            <ComboboxEmpty>No matches</ComboboxEmpty>
            <ComboboxCollection>
              {(item: CityOption) => (
                <ComboboxItem key={item.value} value={item}>
                  {item.label}
                </ComboboxItem>
              )}
            </ComboboxCollection>
          </ComboboxList>
        </ComboboxContent>
      </Combobox>
    </div>
  ),
};

export const Filtering: Story = {
  parameters: {
    docs: {
      description: {
        story: "Type 'al' to narrow the list to items starting with that prefix.",
      },
    },
  },
  render: () => (
    <div className="w-80">
      <Combobox items={cities} defaultOpen>
        <ComboboxInput placeholder="Try typing 'al'" />
        <ComboboxContent>
          <ComboboxList>
            <ComboboxEmpty>No matches</ComboboxEmpty>
            <ComboboxCollection>
              {(item: CityOption) => (
                <ComboboxItem key={item.value} value={item}>
                  {item.label}
                </ComboboxItem>
              )}
            </ComboboxCollection>
          </ComboboxList>
        </ComboboxContent>
      </Combobox>
    </div>
  ),
};

function MultiSelectCombobox() {
  const anchor = useComboboxAnchor();
  return (
    <div className="w-80">
      <Combobox items={cities} multiple>
        <ComboboxChips ref={anchor}>
          <ComboboxChipsInput placeholder="Add cities" aria-label="Add cities" />
        </ComboboxChips>
        <ComboboxContent anchor={anchor}>
          <ComboboxList>
            <ComboboxEmpty>No matches</ComboboxEmpty>
            <ComboboxCollection>
              {(item: CityOption) => (
                <ComboboxItem key={item.value} value={item}>
                  {item.label}
                </ComboboxItem>
              )}
            </ComboboxCollection>
          </ComboboxList>
        </ComboboxContent>
      </Combobox>
    </div>
  );
}

export const MultiSelect: Story = {
  parameters: {
    docs: {
      description: {
        story:
          "`multiple` enables chip-based multi-select. Selections render as `<ComboboxChip>` inside `<ComboboxChips>`; remove with click or Backspace.",
      },
    },
  },
  render: () => <MultiSelectCombobox />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const input = await canvas.findByRole("combobox", { name: "Add cities" });
    await userEvent.click(input);
    await waitFor(() => expect(within(document.body).getByText("Berlin")).toBeInTheDocument());
    await userEvent.click(within(document.body).getByText("Berlin"));
    await waitFor(() => expect(document.querySelector("[data-slot=combobox-chip]")).not.toBeNull());
  },
};

// Re-export to ensure ComboboxChip remains a public primitive exercised by this story file.
void ComboboxChip;
