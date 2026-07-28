import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";
import { overwriteWriteToolMessageFixture, writeToolMessageFixture } from "@/systems/session/mocks";

import { WriteContent } from "../write-content";

const meta: Meta<typeof WriteContent> = {
  title: "systems/session/components/tool-renderers/WriteContent",
  component: WriteContent,
  parameters: {
    layout: "centered",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function WriteFrame({ children }: { children: React.ReactNode }) {
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
    <WriteFrame>
      <WriteContent message={writeToolMessageFixture} />
    </WriteFrame>
  ),
};

export const OverwriteWarning: Story = {
  render: () => (
    <WriteFrame>
      <WriteContent message={overwriteWriteToolMessageFixture} />
    </WriteFrame>
  ),
};
