import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";
import { bashToolMessageFixture, runningBashToolMessageFixture } from "@/systems/session/mocks";

import { ExpandedToolContent } from "../expanded-tool-content";

const meta: Meta<typeof ExpandedToolContent> = {
  title: "systems/session/components/tool-renderers/ExpandedToolContent",
  component: ExpandedToolContent,
  parameters: {
    layout: "centered",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function ExpandedFrame({ children }: { children: React.ReactNode }) {
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
    <ExpandedFrame>
      <ExpandedToolContent message={bashToolMessageFixture} />
    </ExpandedFrame>
  ),
};

export const Running: Story = {
  render: () => (
    <ExpandedFrame>
      <ExpandedToolContent message={runningBashToolMessageFixture} />
    </ExpandedFrame>
  ),
};
