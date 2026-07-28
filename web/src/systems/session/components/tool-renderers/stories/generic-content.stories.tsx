import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";
import { genericToolMessageFixture } from "@/systems/session/mocks";

import { GenericContent } from "../generic-content";

const meta: Meta<typeof GenericContent> = {
  title: "systems/session/components/tool-renderers/GenericContent",
  component: GenericContent,
  parameters: {
    layout: "centered",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  render: () => (
    <CenteredSurface>
      <div className="w-full max-w-3xl rounded-2xl border border-line bg-canvas p-4">
        <GenericContent message={genericToolMessageFixture} />
      </div>
    </CenteredSurface>
  ),
};
