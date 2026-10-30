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
  tags: ["play-fn"],
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

/** A pane inside a window: the host's width decides, and a narrow host is covered edge to edge. */
function ContainedExample({ hostWidth }: { hostWidth: number }) {
  const [open, setOpen] = useState(true);
  const [width, setWidth] = useState(480);
  return (
    <div className="flex h-screen bg-canvas p-6 text-fg">
      <div
        className="relative flex h-full overflow-hidden rounded-lg border border-line"
        style={{ width: hostWidth }}
      >
        <ResizableDrawer
          breakpointBasis="container"
          inlineBreakpoint={760}
          title="Activity"
          open={open}
          onOpenChange={setOpen}
          width={width}
          onWidthChange={setWidth}
          header={
            <div className="flex h-window-head shrink-0 items-center gap-2 border-b border-line px-4">
              <span className="min-w-0 flex-1 truncate text-item-title font-medium">Activity</span>
              <Button size="sm" variant="quiet" onClick={() => setOpen(false)}>
                Close
              </Button>
            </div>
          }
          content={<p className="p-4">The host keeps its own layout beside the pane.</p>}
        >
          <div className="p-4">
            <Button onClick={() => setOpen(true)}>Open activity</Button>
          </div>
        </ResizableDrawer>
      </div>
    </div>
  );
}

export const ContainedWide: Story = {
  tags: ["play-fn"],
  render: () => <ContainedExample hostWidth={1100} />,
  play: async ({ canvasElement }) => {
    const dialog = await within(canvasElement).findByRole("dialog", { name: "Activity" });
    await expect(dialog).toHaveAttribute("data-mode", "inline");
  },
};

export const ContainedNarrow: Story = { render: () => <ContainedExample hostWidth={560} /> };
