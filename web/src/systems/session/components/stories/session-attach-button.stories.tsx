import type { Meta, StoryObj } from "@storybook/react-vite";

import { SessionAttachButtonView } from "@/components/assistant-ui/session-attach-button";
import { CenteredSurface } from "@/storybook/story-layout";

const meta: Meta<typeof SessionAttachButtonView> = {
  title: "systems/session/components/SessionAttachButton",
  component: SessionAttachButtonView,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component: "26×26 paperclip control that opens the composer file picker.",
      },
    },
  },
  decorators: [
    Story => (
      <CenteredSurface>
        <div className="flex items-center gap-2 rounded-lg border border-line bg-elevated p-2">
          <Story />
        </div>
      </CenteredSurface>
    ),
  ],
  args: {
    disabled: false,
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const Disabled: Story = {
  args: { disabled: true },
};
