import type { ReactNode } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";
import { ReadContent } from "@/systems/session/components/tool-renderers/read-content";
import { readToolMessageFixture, truncatedReadToolMessageFixture } from "@/systems/session/mocks";

const meta: Meta<typeof ReadContent> = {
  title: "systems/session/components/tool-renderers/ReadContent",
  component: ReadContent,
  parameters: {
    layout: "centered",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

interface ReadFrameProps {
  children: ReactNode;
}

function ReadFrame({ children }: ReadFrameProps) {
  return (
    <CenteredSurface>
      <div className="w-full max-w-3xl rounded-2xl border border-line bg-canvas p-4">
        {children}
      </div>
    </CenteredSurface>
  );
}

export const Default: Story = {
  args: {},
  render: () => (
    <ReadFrame>
      <ReadContent message={readToolMessageFixture} />
    </ReadFrame>
  ),
};

export const Truncated: Story = {
  args: {},
  render: () => (
    <ReadFrame>
      <ReadContent message={truncatedReadToolMessageFixture} />
    </ReadFrame>
  ),
};
