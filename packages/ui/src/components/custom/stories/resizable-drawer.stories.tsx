import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, within } from "storybook/test";
import { useState } from "react";
import { Button } from "../../button";
import { ResizableDrawer } from "../resizable-drawer";

const meta: Meta<typeof ResizableDrawer> = {
  title: "components/custom/ResizableDrawer",
  component: ResizableDrawer,
  parameters: { layout: "fullscreen" },
};
export default meta;
type Story = StoryObj<typeof meta>;

function DrawerExample({ inlineBreakpoint }: { inlineBreakpoint?: number }) {
  const [open, setOpen] = useState(true);
  const [width, setWidth] = useState(480);
  return (
    <div className="flex h-screen bg-canvas text-fg">
      <ResizableDrawer
        title="Activity"
        inlineBreakpoint={inlineBreakpoint}
        open={open}
        onOpenChange={setOpen}
        width={width}
        onWidthChange={setWidth}
        content={<p className="p-4">Read the activity while keeping the conversation in view.</p>}
      >
        <div className="p-4">
          <Button onClick={() => setOpen(true)}>Open activity</Button>
        </div>
      </ResizableDrawer>
    </div>
  );
}

export const Responsive: Story = { render: () => <DrawerExample /> };

export const Inline: Story = {
  render: () => <DrawerExample inlineBreakpoint={0} />,
  play: async ({ canvasElement }) => {
    const separator = await within(canvasElement).findByRole("separator", {
      name: "Resize drawer",
    });
    separator.focus();
    await userEvent.keyboard("{ArrowLeft}");
    await expect(separator).toHaveAttribute("aria-valuenow", "496");
    const bounds = separator.getBoundingClientRect();
    await userEvent.pointer([
      {
        target: separator,
        keys: "[MouseLeft>]",
        coords: { clientX: bounds.x, clientY: bounds.y + 40 },
      },
      { target: separator, coords: { clientX: bounds.x - 32, clientY: bounds.y + 40 } },
      { target: separator, keys: "[/MouseLeft]" },
    ]);
    await expect(separator).toHaveAttribute("aria-valuenow", "528");
  },
};
export const Overlay: Story = { render: () => <DrawerExample inlineBreakpoint={10000} /> };
