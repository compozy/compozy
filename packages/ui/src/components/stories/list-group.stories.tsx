import type { Meta, StoryObj } from "@storybook/react-vite";
import { Plus } from "lucide-react";

import { Button } from "../button";
import { Item, ItemContent, ItemDescription, ItemTitle } from "../item";
import { ListGroup } from "../custom/list-group";

const meta: Meta<typeof ListGroup> = {
  title: "components/custom/ListGroup",
  component: ListGroup,
  parameters: {
    layout: "centered",
  },
  decorators: [
    Story => (
      <div className="w-[440px] overflow-hidden rounded-lg border border-line bg-canvas-soft">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

export const LabelAndCount: Story = {
  args: {},
  render: () => (
    <ListGroup count={2} label="Global">
      <Item className="rounded-none border-x-0 border-t-0">
        <ItemContent>
          <ItemTitle>Operator Style</ItemTitle>
          <ItemDescription>Guidance for calm operator communication.</ItemDescription>
        </ItemContent>
      </Item>
      <Item className="rounded-none border-x-0 border-t-0">
        <ItemContent>
          <ItemTitle>Launch Brief</ItemTitle>
          <ItemDescription>Canonical launch narrative and owners.</ItemDescription>
        </ItemContent>
      </Item>
    </ListGroup>
  ),
};

export const LabelAndActions: Story = {
  args: {},
  render: () => (
    <ListGroup
      actions={
        <Button size="icon-xs" type="button" variant="ghost">
          <Plus />
          <span className="sr-only">Add</span>
        </Button>
      }
      count={1}
      label="Workspace"
    >
      <Item className="rounded-none border-x-0 border-t-0">
        <ItemContent>
          <ItemTitle>Executive Risk Memo</ItemTitle>
          <ItemDescription>Notes on launch blockers and fallback paths.</ItemDescription>
        </ItemContent>
      </Item>
    </ListGroup>
  ),
};

export const CompoundParts: Story = {
  args: {},
  render: () => (
    <ListGroup.Root>
      <ListGroup.Header count={1} label="Agent" />
      <ListGroup.Items>
        <Item className="rounded-none border-x-0 border-t-0">
          <ItemContent>
            <ItemTitle>CTO Tone</ItemTitle>
            <ItemDescription>Direct, calm tone for CTO-facing summaries.</ItemDescription>
          </ItemContent>
        </Item>
      </ListGroup.Items>
    </ListGroup.Root>
  ),
};
