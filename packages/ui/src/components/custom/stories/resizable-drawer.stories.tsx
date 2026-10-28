import type { Meta, StoryObj } from "@storybook/react-vite";
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

function DrawerExample() {
  const [open, setOpen] = useState(true);
  const [width, setWidth] = useState(480);
  return (
    <div className="flex h-screen bg-canvas text-fg">
      <ResizableDrawer
        title="Activity"
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
