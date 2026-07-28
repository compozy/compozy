import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";
import { editToolMessageFixture, multiHunkEditToolMessageFixture } from "@/systems/session/mocks";

import { EditContent } from "../edit-content";

const meta: Meta<typeof EditContent> = {
  title: "systems/session/components/tool-renderers/EditContent",
  component: EditContent,
  parameters: {
    layout: "centered",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function EditFrame({ children }: { children: React.ReactNode }) {
  return (
    <CenteredSurface>
      <div className="w-full max-w-3xl rounded-2xl border border-line bg-canvas p-4">
        {children}
      </div>
    </CenteredSurface>
  );
}

export const Default: Story = {
  render: () => (
    <EditFrame>
      <EditContent message={editToolMessageFixture} />
    </EditFrame>
  ),
};

export const MultiHunk: Story = {
  render: () => (
    <EditFrame>
      <EditContent message={multiHunkEditToolMessageFixture} />
    </EditFrame>
  ),
};
