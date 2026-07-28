import type { Meta, StoryObj } from "@storybook/react-vite";
import {
  AlignCenterIcon,
  AlignJustifyIcon,
  AlignLeftIcon,
  AlignRightIcon,
  BoldIcon,
  ItalicIcon,
  UnderlineIcon,
} from "lucide-react";

import { ToggleGroup, ToggleGroupItem } from "../toggle-group";

const meta: Meta<typeof ToggleGroup> = {
  title: "components/ui/ToggleGroup",
  component: ToggleGroup,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Cluster of toggles sharing a value. `multiple` controls whether one or several items may be pressed at once.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const SingleSelection: Story = {
  render: () => (
    <ToggleGroup defaultValue={["left"]}>
      <ToggleGroupItem value="left" aria-label="Align left">
        <AlignLeftIcon />
      </ToggleGroupItem>
      <ToggleGroupItem value="center" aria-label="Align center">
        <AlignCenterIcon />
      </ToggleGroupItem>
      <ToggleGroupItem value="right" aria-label="Align right">
        <AlignRightIcon />
      </ToggleGroupItem>
      <ToggleGroupItem value="justify" aria-label="Justify">
        <AlignJustifyIcon />
      </ToggleGroupItem>
    </ToggleGroup>
  ),
};

export const MultiSelection: Story = {
  parameters: {
    docs: {
      description: {
        story: "With `multiple`, bold and italic are both pressable at the same time.",
      },
    },
  },
  render: () => (
    <ToggleGroup multiple defaultValue={["bold", "italic"]} variant="outline">
      <ToggleGroupItem value="bold" aria-label="Bold">
        <BoldIcon />
      </ToggleGroupItem>
      <ToggleGroupItem value="italic" aria-label="Italic">
        <ItalicIcon />
      </ToggleGroupItem>
      <ToggleGroupItem value="underline" aria-label="Underline">
        <UnderlineIcon />
      </ToggleGroupItem>
    </ToggleGroup>
  ),
};

export const Vertical: Story = {
  render: () => (
    <ToggleGroup orientation="vertical" defaultValue={["left"]} variant="outline">
      <ToggleGroupItem value="left" aria-label="Align left">
        <AlignLeftIcon />
      </ToggleGroupItem>
      <ToggleGroupItem value="center" aria-label="Align center">
        <AlignCenterIcon />
      </ToggleGroupItem>
      <ToggleGroupItem value="right" aria-label="Align right">
        <AlignRightIcon />
      </ToggleGroupItem>
    </ToggleGroup>
  ),
};
