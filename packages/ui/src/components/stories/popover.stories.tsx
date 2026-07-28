import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, waitFor, within } from "storybook/test";

import {
  Popover,
  PopoverContent,
  PopoverDescription,
  PopoverHeader,
  PopoverTitle,
  PopoverTrigger,
} from "../popover";
import { Button } from "../button";
import { Input } from "../input";
import { Label } from "../label";
import { UIProvider } from "../custom/ui-provider";

const meta: Meta<typeof Popover> = {
  title: "components/ui/Popover",
  component: Popover,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Floating surface for contextual actions and quick forms. Positioning is delegated to Base UI's collision-aware Positioner; enter/exit animations run through motion's `AnimatePresence`.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function QuickRenamePopover() {
  return (
    <Popover>
      <PopoverTrigger render={<Button variant="outline">Open popover</Button>} />
      <PopoverContent>
        <PopoverHeader>
          <PopoverTitle>Quick rename</PopoverTitle>
          <PopoverDescription>Changes apply only to this row.</PopoverDescription>
        </PopoverHeader>
        <div className="grid gap-2">
          <Label htmlFor="popover-name">Label</Label>
          <Input id="popover-name" defaultValue="daemon-us-east" />
        </div>
        <Button size="sm">Save</Button>
      </PopoverContent>
    </Popover>
  );
}

export const Default: Story = {
  render: () => <QuickRenamePopover />,
};

export const RightAligned: Story = {
  render: () => (
    <Popover>
      <PopoverTrigger render={<Button variant="ghost">Show context</Button>} />
      <PopoverContent align="end" side="right">
        <PopoverHeader>
          <PopoverTitle>Run metadata</PopoverTitle>
        </PopoverHeader>
        <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-xs">
          <dt className="text-muted-foreground">Session</dt>
          <dd className="font-mono">s-0194</dd>
          <dt className="text-muted-foreground">Agent</dt>
          <dd>claude-code</dd>
          <dt className="text-muted-foreground">Duration</dt>
          <dd>8m 21s</dd>
        </dl>
      </PopoverContent>
    </Popover>
  ),
};

export const ReducedMotion: Story = {
  parameters: {
    docs: {
      description: {
        story:
          "With `UIProvider reducedMotion='always'`, motion drops the scale transform and only opacity animates.",
      },
    },
  },
  render: () => (
    <UIProvider reducedMotion="always">
      <QuickRenamePopover />
    </UIProvider>
  ),
};

export const OpenAndClose: Story = {
  render: () => <QuickRenamePopover />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const trigger = await canvas.findByRole("button", { name: "Open popover" });
    await userEvent.click(trigger);
    const title = await waitFor(() => within(document.body).getByText("Quick rename"));
    await expect(title).toBeInTheDocument();
    await userEvent.keyboard("{Escape}");
    await waitFor(
      () => expect(within(document.body).queryByText("Quick rename")).not.toBeInTheDocument(),
      { timeout: 2000 }
    );
  },
};
