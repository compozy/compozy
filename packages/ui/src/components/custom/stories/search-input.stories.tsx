import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, userEvent, waitFor, within } from "storybook/test";

import { Kbd } from "../../kbd";
import { SearchInput } from "../search-input";

const meta: Meta<typeof SearchInput> = {
  title: "components/custom/SearchInput",
  component: SearchInput,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Search field with leading glyph, optional kbd hint slot, and standard 36px row.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function Harness(props: { initial?: string; placeholder?: string; kbd?: React.ReactNode }) {
  const [value, setValue] = useState(props.initial ?? "");
  return (
    <SearchInput
      value={value}
      onChange={setValue}
      placeholder={props.placeholder}
      kbd={props.kbd}
      aria-label="Search"
      containerClassName="w-72"
    />
  );
}

export const Basic: Story = {
  render: () => <Harness placeholder="Search sessions..." />,
};

export const WithKbdHint: Story = {
  render: () => <Harness placeholder="Search..." kbd={<Kbd>⌘K</Kbd>} />,
};

export const Typing: Story = {
  render: () => <Harness placeholder="Search workspaces" />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const input = await canvas.findByPlaceholderText("Search workspaces");
    await userEvent.type(input, "agh");
    await waitFor(() => expect((input as HTMLInputElement).value).toBe("agh"));
  },
};
