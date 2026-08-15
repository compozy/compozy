import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";

import { SessionAttachmentFrame } from "../session-attachment-frame";

const STORY_IMAGE =
  "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==";

const meta: Meta<typeof SessionAttachmentFrame> = {
  title: "systems/session/components/SessionAttachmentFrame",
  component: SessionAttachmentFrame,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Fixed 280px transcript image frame. Caption appears on hover or focus-visible only. Click opens the bytes URL.",
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
    href: "#att-diagram",
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
