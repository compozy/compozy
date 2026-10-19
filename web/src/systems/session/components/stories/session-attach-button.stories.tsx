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
        component:
          '28 px square paperclip tool (`InputGroupButton size="tool-icon"`) that opens the composer file picker.',
      },
    },
  },
  decorators: [
    Story => (
      <CenteredSurface>
        <div className="flex items-center gap-2 rounded-lg bg-canvas shadow-card p-2">
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
