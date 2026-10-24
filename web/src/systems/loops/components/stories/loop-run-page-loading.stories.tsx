import type { Meta, StoryObj } from "@storybook/react-vite";

import { StoryTopbarHost } from "@/storybook/story-layout";

import { LoopRunPageSkeleton } from "../../index";

const meta: Meta<typeof LoopRunPageSkeleton> = {
  title: "systems/loops/components/LoopRunPageLoading",
  component: LoopRunPageSkeleton,
  parameters: { layout: "fullscreen" },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** The run record has not answered: the page's own frame over placeholders. */
export const Loading: Story = {
  args: {},
  render: () => (
    <div className="flex h-dvh flex-col bg-canvas">
      <StoryTopbarHost title="Loops">
        <LoopRunPageSkeleton />
      </StoryTopbarHost>
    </div>
  ),
};
