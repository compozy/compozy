import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { CenteredSurface } from "@/storybook/story-layout";

import { SessionStopAttentionNotice } from "../session-stop-attention-notice";

const meta: Meta<typeof SessionStopAttentionNotice> = {
  title: "systems/session/components/SessionStopAttentionNotice",
  component: SessionStopAttentionNotice,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Inline warning shown while a session stays `stopping` because the daemon ran the whole stop ladder and still could not verify the agent process is gone. The only action is the same session stop, again; the notice clears only when the daemon reads `stopped`.",
      },
    },
  },
  decorators: [
    Story => (
      <CenteredSurface>
        <div className="w-full max-w-3xl">
          <Story />
        </div>
      </CenteredSurface>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/** The unverified stop, waiting on the operator. */
export const NeedsAttention: Story = {
  args: {
    isRetrying: false,
    onRetry: fn(),
  },
};

/** A retry is on the wire or still landing: the action holds with a spinner. */
export const Retrying: Story = {
  args: {
    isRetrying: true,
    onRetry: fn(),
  },
};

/** A session the operator may not stop (managed lifecycle) reads the state without an action. */
export const WithoutAction: Story = {
  args: {
    isRetrying: false,
  },
};
