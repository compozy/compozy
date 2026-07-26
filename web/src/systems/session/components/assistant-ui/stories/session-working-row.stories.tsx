import type { Meta, StoryObj } from "@storybook/react-vite";

import { WorkingIndicator } from "@/components/assistant-ui/session-working-row";

// Fixed five seconds before the story mounts, so the live timer reads "5s"+ at
// capture time without waiting for the clock to advance.
const STARTED_AT = Date.now() - 5000;

/**
 * Streaming indicator shown while an assistant turn is in flight.
 *
 * Motion: typing dots (wiring the `typing-bounce` keyframe on `bg-subtle` dots)
 * beside a self-ticking "Working for Xs" `tabular-nums` timer. Reduced motion
 * degrades to a resting label with no animation classes.
 */
const meta: Meta<typeof WorkingIndicator> = {
  title: "systems/session/components/assistant-ui/SessionWorkingRow",
  component: WorkingIndicator,
  parameters: { layout: "centered" },
  decorators: [
    Story => (
      <div className="flex w-80 flex-col gap-4 border border-line bg-background p-6">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Live turn: bouncing typing dots + counting "Working for Xs" timer. */
export const Motion: Story = {
  args: { startedAt: STARTED_AT, reducedMotion: false },
};

/** `prefers-reduced-motion`: static resting label, no dots and no ticking timer. */
export const ReducedMotion: Story = {
  args: { startedAt: STARTED_AT, reducedMotion: true },
};

/** Pre-first-token turn with no known start time: dots beside a static label. */
export const PreFirstToken: Story = {
  args: { reducedMotion: false },
};
