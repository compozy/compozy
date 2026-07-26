import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";
import { bashToolMessageFixture, longBashToolMessageFixture } from "@/systems/session/mocks";

import { BashContent } from "../bash-content";

const meta: Meta<typeof BashContent> = {
  title: "systems/session/components/tool-renderers/BashContent",
  component: BashContent,
  parameters: {
    layout: "centered",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function BashFrame({ children }: { children: React.ReactNode }) {
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
    <BashFrame>
      <BashContent message={bashToolMessageFixture} />
    </BashFrame>
  ),
};

export const LongOutput: Story = {
  render: () => (
    <BashFrame>
      <BashContent message={longBashToolMessageFixture} />
    </BashFrame>
  ),
};
