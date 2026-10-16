import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";

import { SessionAttachmentFrame } from "../session-attachment-frame";

const STORY_IMAGE = "/icon-512.png";

const meta: Meta<typeof SessionAttachmentFrame> = {
  title: "systems/session/components/SessionAttachmentFrame",
  component: SessionAttachmentFrame,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Fixed 280px transcript image frame. Caption appears on hover or focus-visible only. Click opens a dismissible image preview without leaving the session.",
      },
    },
  },
  decorators: [
    Story => (
      <CenteredSurface>
        <div className="bg-canvas p-6">
          <Story />
        </div>
      </CenteredSurface>
    ),
  ],
  args: {
    src: STORY_IMAGE,
    filename: "diagram.png",
    title: "diagram.png · 1280×720",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Rest state — caption hidden. */
export const Default: Story = {
  args: {},
};

/** Long filename truncates inside the caption chip. */
export const LongFilename: Story = {
  args: {
    filename: "northstar-pay-checkout-flow-annotated-v3.png",
    title: "northstar-pay-checkout-flow-annotated-v3.png · 1920×1080",
  },
};
