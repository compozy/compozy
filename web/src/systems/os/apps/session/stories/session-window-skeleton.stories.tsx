import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";

import { SessionWindowSkeleton } from "../session-window-skeleton";

const meta = {
  title: "systems/os/components/SessionWindowSkeleton",
  component: SessionWindowSkeleton,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "A session pane whose session has not loaded: a transcript over a composer card, in the frame the loaded pane fills, so the swap moves nothing. It mounts inside the window's live sessions rail, which stays put across a session switch. One busy status for the pane; no queries, so hidden windows load cheaply.",
      },
    },
  },
  decorators: [
    Story => (
      <CenteredSurface>
        <div className="flex h-[560px] w-[720px] overflow-hidden rounded-lg border border-line-strong bg-canvas">
          <Story />
        </div>
      </CenteredSurface>
    ),
  ],
} satisfies Meta<typeof SessionWindowSkeleton>;

export default meta;
type Story = StoryObj<typeof meta>;

/** The transcript and composer placeholders the loaded pane replaces. */
export const Default: Story = {};
